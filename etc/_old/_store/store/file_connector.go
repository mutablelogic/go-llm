package store

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	// Packages

	schema "github.com/mutablelogic/go-llm/kernel/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// FileConnectorStore is a filesystem-backed implementation of ConnectorStore.
// Each connector is stored as a separate JSON file in a directory, keyed by
// a SHA-256 hash of the canonical URL. It is safe for concurrent use.
type FileConnectorStore struct {
	mu  sync.RWMutex
	dir string
}

var _ schema.ConnectorStore = (*FileConnectorStore)(nil)

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// NewFileConnectorStore creates a new file-backed connector store rooted at dir.
// The directory is created (with parents) if it does not already exist.
func NewFileConnectorStore(dir string) (*FileConnectorStore, error) {
	if err := ensureDir(dir); err != nil {
		return nil, err
	}
	return &FileConnectorStore{dir: dir}, nil
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// CreateConnector registers a new MCP server connector keyed by insert.URL.
// Returns an error if the URL is invalid, namespace is invalid, or a connector
// with that URL or namespace already exists.
func (s *FileConnectorStore) CreateConnector(_ context.Context, insert schema.ConnectorInsert) (*schema.Connector, error) {
	canonicalURL, err := schema.CanonicalURL(insert.URL)
	if err != nil {
		return nil, err
	}
	if err := validateConnectorNamespace(types.Value(insert.Namespace)); err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	path := s.path(canonicalURL)
	if _, err := os.Stat(path); err == nil {
		return nil, schema.ErrConflict.Withf("connector already exists for %q", canonicalURL)
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("stat %q: %w", path, err)
	}
	if ns := types.Value(insert.Namespace); ns != "" {
		matches, err := s.filterConnectors(schema.ConnectorListRequest{Namespace: ns}, "")
		if err != nil {
			return nil, err
		}
		if len(matches) > 0 {
			return nil, schema.ErrConflict.Withf("connector namespace %q already in use by %q", ns, matches[0].URL)
		}
	}

	c := schema.Connector{
		URL:           canonicalURL,
		CreatedAt:     time.Now(),
		ConnectorMeta: insert.ConnectorMeta,
	}
	if err := writeJSON(path, &c); err != nil {
		return nil, err
	}
	return &c, nil
}

// GetConnector returns the connector for the given URL, or ErrNotFound.
func (s *FileConnectorStore) GetConnector(_ context.Context, url string) (*schema.Connector, error) {
	canonicalURL, err := schema.CanonicalURL(url)
	if err != nil {
		return nil, err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.read(canonicalURL)
}

// UpdateConnector applies meta to the connector identified by url.
// Only the user-editable ConnectorMeta fields are updated (nil fields are preserved).
// Returns ErrBadParameter if namespace is invalid, ErrNotFound if the connector does not exist.
func (s *FileConnectorStore) UpdateConnector(_ context.Context, url string, meta schema.ConnectorMeta) (*schema.Connector, error) {
	canonicalURL, err := schema.CanonicalURL(url)
	if err != nil {
		return nil, err
	}
	if err := validateConnectorNamespace(types.Value(meta.Namespace)); err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	c, err := s.read(canonicalURL)
	if err != nil {
		return nil, err
	}
	if ns := types.Value(meta.Namespace); ns != "" {
		matches, err := s.filterConnectors(schema.ConnectorListRequest{Namespace: ns}, canonicalURL)
		if err != nil {
			return nil, err
		}
		if len(matches) > 0 {
			return nil, schema.ErrConflict.Withf("connector namespace %q already in use by %q", ns, matches[0].URL)
		}
	}
	if meta.Enabled != nil {
		c.Enabled = meta.Enabled
	}
	if meta.Namespace != nil {
		c.Namespace = meta.Namespace
	}
	if meta.Meta != nil {
		c.Meta = meta.Meta
	}
	if err := writeJSON(s.path(canonicalURL), c); err != nil {
		return nil, err
	}
	return c, nil
}

// DeleteConnector removes the connector for the given URL.
// Returns ErrNotFound if the connector does not exist.
func (s *FileConnectorStore) DeleteConnector(_ context.Context, url string) error {
	canonicalURL, err := schema.CanonicalURL(url)
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	path := s.path(canonicalURL)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return schema.ErrNotFound.Withf("connector not found for %q", canonicalURL)
	}
	if err := os.Remove(path); err != nil {
		return schema.ErrInternalServerError.Withf("remove: %v", err)
	}
	return nil
}

// ListConnectors returns connectors matching the filters in req.
// The Namespace and Enabled filters are applied first; pagination (Offset/Limit)
// is applied to the filtered slice. The Count field reflects the total number
// of matching connectors before pagination.
func (s *FileConnectorStore) ListConnectors(_ context.Context, req schema.ConnectorListRequest) (*schema.LConnectorList error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	matched, err := s.filterConnectors(req, "")
	if err != nil {
		return nil, err
	}

	sort.Slice(matched, func(i, j int) bool {
		return matched[i].CreatedAt.After(matched[j].CreatedAt)
	})

	total := uint(len(matched))

	if req.Offset > 0 {
		if req.Offset >= total {
			matched = nil
		} else {
			matched = matched[req.Offset:]
		}
	}
	if req.Limit != nil && uint(len(matched)) > *req.Limit {
		matched = matched[:*req.Limit]
	}

	body := make([]*schema.Connector, len(matched))
	for i := range matched {
		body[i] = types.Ptr(matched[i])
	}
	return &schema.ConnectorList{
		Count:  total,
		Offset: req.Offset,
		Limit:  req.Limit,
		Body:   body,
	}, nil
}

// UpdateConnectorState merges state into the connector identified by url.
// Returns ErrNotFound if the connector does not exist.
func (s *FileConnectorStore) UpdateConnectorState(_ context.Context, url string, state schema.ConnectorState) (*schema.Connector, error) {
	canonicalURL, err := schema.CanonicalURL(url)
	if err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	c, err := s.read(canonicalURL)
	if err != nil {
		return nil, err
	}

	if state.ConnectedAt != nil {
		c.ConnectedAt = state.ConnectedAt
	}
	if state.Name != nil {
		c.Name = state.Name
	}
	if state.Title != nil {
		c.Title = state.Title
	}
	if state.Description != nil {
		c.Description = state.Description
	}
	if state.Version != nil {
		c.Version = state.Version
	}
	if state.Capabilities != nil {
		c.Capabilities = state.Capabilities
	}

	if err := writeJSON(s.path(canonicalURL), c); err != nil {
		return nil, err
	}
	return c, nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// path returns the filesystem path for a given canonical URL.
func (s *FileConnectorStore) path(canonicalURL string) string {
	return hashPath(s.dir, canonicalURL, jsonExt)
}

// read deserialises a connector from its JSON file.
// Caller must hold at least s.mu.RLock.
func (s *FileConnectorStore) read(canonicalURL string) (*schema.Connector, error) {
	var c schema.Connector
	if err := readJSON(s.path(canonicalURL), fmt.Sprintf("connector not found for %q", canonicalURL), &c); err != nil {
		return nil, err
	}
	return &c, nil
}

// filterConnectors loads all connectors from disk, applies req filters, and
// skips the connector at excludeURL (pass "" to include all).
// Pagination fields in req are ignored. Corrupt files are silently skipped.
// Caller must hold at least s.mu.RLock.
func (s *FileConnectorStore) filterConnectors(req schema.ConnectorListRequest, excludeURL string) ([]schema.Connector, error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, fmt.Errorf("readdir %q: %w", s.dir, err)
	}
	var matched []schema.Connector
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != jsonExt {
			continue
		}
		var c schema.Connector
		path := filepath.Join(s.dir, entry.Name())
		if err := readJSON(path, "", &c); err != nil {
			continue // skip corrupt files
		}
		if c.URL == excludeURL {
			continue
		}
		if req.Namespace != "" && types.Value(c.Namespace) != req.Namespace {
			continue
		}
		if req.Enabled != nil && types.Value(c.Enabled) != types.Value(req.Enabled) {
			continue
		}
		matched = append(matched, c)
	}
	return matched, nil
}
