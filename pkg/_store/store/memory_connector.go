package store

import (
	"context"
	"sort"
	"sync"
	"time"

	// Packages

	schema "github.com/mutablelogic/go-llm/pkg/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// MemoryConnectorStore is an in-memory implementation of ConnectorStore.
// It is safe for concurrent use. State is lost when the process exits.
type MemoryConnectorStore struct {
	mu         sync.RWMutex
	connectors map[string]schema.Connector // keyed by URL
}

var _ schema.ConnectorStore = (*MemoryConnectorStore)(nil)

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// NewMemoryConnectorStore returns a new empty in-memory connector store.
func NewMemoryConnectorStore() *MemoryConnectorStore {
	return &MemoryConnectorStore{
		connectors: make(map[string]schema.Connector),
	}
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// CreateConnector registers a new MCP server connector keyed by insert.URL.
// Returns an error if the URL is invalid, namespace is invalid, or a connector
// with that URL already exists.
func (s *MemoryConnectorStore) CreateConnector(_ context.Context, insert schema.ConnectorInsert) (*schema.Connector, error) {
	// Validate parameters before acquiring the lock.
	canonicalURL, err := schema.CanonicalURL(insert.URL)
	if err != nil {
		return nil, err
	}
	if err := validateConnectorNamespace(types.Value(insert.Namespace)); err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.connectors[canonicalURL]; ok {
		return nil, schema.ErrConflict.Withf("connector already exists for %q", canonicalURL)
	}
	if ns := types.Value(insert.Namespace); ns != "" {
		if matches := s.listConnectors(schema.ConnectorListRequest{Namespace: ns}, ""); len(matches) > 0 {
			return nil, schema.ErrConflict.Withf("connector namespace %q already in use by %q", ns, matches[0].URL)
		}
	}

	// Create and store the new connector
	c := schema.Connector{
		URL:           canonicalURL,
		CreatedAt:     time.Now(),
		ConnectorMeta: insert.ConnectorMeta,
	}
	s.connectors[canonicalURL] = c
	return types.Ptr(c), nil
}

// GetConnector returns the connector for the given URL, or ErrNotFound.
func (s *MemoryConnectorStore) GetConnector(_ context.Context, url string) (*schema.Connector, error) {
	canonicalURL, err := schema.CanonicalURL(url)
	if err != nil {
		return nil, err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	c, ok := s.connectors[canonicalURL]
	if !ok {
		return nil, schema.ErrNotFound.Withf("connector not found for %q", canonicalURL)
	}
	return types.Ptr(c), nil
}

// UpdateConnector applies meta to the connector identified by url.
// Only the user-editable ConnectorMeta fields are updated.
// Returns ErrBadParameter if namespace is invalid, ErrNotFound if the connector does not exist.
func (s *MemoryConnectorStore) UpdateConnector(_ context.Context, url string, meta schema.ConnectorMeta) (*schema.Connector, error) {
	canonicalURL, err := schema.CanonicalURL(url)
	if err != nil {
		return nil, err
	}
	if err := validateConnectorNamespace(types.Value(meta.Namespace)); err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	c, ok := s.connectors[canonicalURL]
	if !ok {
		return nil, schema.ErrNotFound.Withf("connector not found for %q", canonicalURL)
	}
	if ns := types.Value(meta.Namespace); ns != "" {
		if matches := s.listConnectors(schema.ConnectorListRequest{Namespace: ns}, canonicalURL); len(matches) > 0 {
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
	s.connectors[canonicalURL] = c
	return types.Ptr(c), nil
}

// DeleteConnector removes the connector for the given URL.
// Returns ErrNotFound if the connector does not exist.
func (s *MemoryConnectorStore) DeleteConnector(_ context.Context, url string) error {
	canonicalURL, err := schema.CanonicalURL(url)
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.connectors[canonicalURL]; !ok {
		return schema.ErrNotFound.Withf("connector not found for %q", canonicalURL)
	}
	delete(s.connectors, canonicalURL)
	return nil
}

// ListConnectors returns connectors matching the filters in req.
// The Namespace and Enabled filters are applied first; pagination (Offset/Limit)
// is applied to the filtered slice. The Count field in the response reflects
// the total number of matching connectors before pagination.
func (s *MemoryConnectorStore) ListConnectors(_ context.Context, req schema.ConnectorListRequest) (*schema.ConnectorList, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	matched := s.listConnectors(req, "")

	// Sort by creation time, most recent first.
	sort.Slice(matched, func(i, j int) bool {
		return matched[i].CreatedAt.After(matched[j].CreatedAt)
	})

	total := uint(len(matched))

	// Apply offset.
	if req.Offset > 0 {
		if req.Offset >= total {
			matched = nil
		} else {
			matched = matched[req.Offset:]
		}
	}

	// Apply limit.
	if req.Limit != nil && uint(len(matched)) > *req.Limit {
		matched = matched[:*req.Limit]
	}

	// Build response with pointer copies of each item.
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

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// listConnectors filters s.connectors by req, skipping the connector at
// excludeURL (pass "" to include all). Pagination fields in req are ignored.
// Caller must hold at least s.mu.RLock.
func (s *MemoryConnectorStore) listConnectors(req schema.ConnectorListRequest, excludeURL string) []schema.Connector {
	var matched []schema.Connector
	for url, c := range s.connectors {
		if url == excludeURL {
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
	return matched
}

// UpdateConnectorState merges state into the connector identified by url,
// updating only non-nil pointer fields and replacing Capabilities when
// the incoming slice is non-nil. Returns ErrNotFound if the connector does not exist.
func (s *MemoryConnectorStore) UpdateConnectorState(_ context.Context, url string, state schema.ConnectorState) (*schema.Connector, error) {
	canonicalURL, err := schema.CanonicalURL(url)
	if err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	c, ok := s.connectors[canonicalURL]
	if !ok {
		return nil, schema.ErrNotFound.Withf("connector not found for %q", canonicalURL)
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

	s.connectors[canonicalURL] = c
	return types.Ptr(c), nil
}
