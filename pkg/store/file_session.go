package store

import (
	"context"
	"fmt"
	"os"
	"sort"
	"sync"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// FileSessionStore is a file-backed implementation of Store.
// Each session is stored as {id}.json in a directory.
// It is safe for concurrent use.
type FileSessionStore struct {
	mu  sync.RWMutex
	dir string
}

var _ schema.SessionStore = (*FileSessionStore)(nil)

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// NewFileSessionStore creates a new file-backed session store in the given directory.
// The directory is created if it does not exist.
func NewFileSessionStore(dir string) (*FileSessionStore, error) {
	if err := ensureDir(dir); err != nil {
		return nil, err
	}
	return &FileSessionStore{dir: dir}, nil
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// CreateSession creates a new session with a unique ID, writes it to disk,
// and returns it.
func (f *FileSessionStore) CreateSession(_ context.Context, meta schema.SessionMeta) (*schema.Session, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	s, err := newSession(meta)
	if err != nil {
		return nil, err
	}
	if err := f.write(s); err != nil {
		return nil, err
	}
	return s, nil
}

// GetSession retrieves a session by ID from disk.
func (f *FileSessionStore) GetSession(_ context.Context, id string) (*schema.Session, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	return f.read(id)
}

// ListSessions returns sessions from disk, ordered by last modified time
// (most recent first), with pagination support.
func (f *FileSessionStore) ListSessions(_ context.Context, req schema.ListSessionRequest) (*schema.ListSessionResponse, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	ids, err := readJSONDir(f.dir)
	if err != nil {
		return nil, err
	}

	result := make([]*schema.Session, 0, len(ids))
	for _, id := range ids {
		s, err := f.read(id)
		if err != nil {
			continue // skip corrupt files
		}
		if !matchLabels(s.Labels, req.Label) {
			continue
		}
		result = append(result, s)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Modified.After(result[j].Modified)
	})

	body, total := paginate(result, req.Offset, req.Limit)
	return &schema.ListSessionResponse{
		Count:  total,
		Offset: req.Offset,
		Limit:  req.Limit,
		Body:   body,
	}, nil
}

// DeleteSession removes a session file by ID.
func (f *FileSessionStore) DeleteSession(_ context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	path := jsonPath(f.dir, id)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return llm.ErrNotFound.Withf("session %q", id)
	}
	if err := os.Remove(path); err != nil {
		return llm.ErrInternalServerError.Withf("remove: %v", err)
	}
	return nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// write serialises a session to its JSON file.
func (f *FileSessionStore) write(s *schema.Session) error {
	return writeJSON(jsonPath(f.dir, s.ID), s)
}

// read deserialises a session from its JSON file.
func (f *FileSessionStore) read(id string) (*schema.Session, error) {
	var s schema.Session
	if err := readJSON(jsonPath(f.dir, id), fmt.Sprintf("session %q", id), &s); err != nil {
		return nil, err
	}
	return &s, nil
}

// WriteSession persists a session's current state to disk.
// This is called after mutations (e.g. Append) to keep the file in sync.
func (f *FileSessionStore) WriteSession(s *schema.Session) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.write(s)
}

// UpdateSession applies non-zero fields from meta to the session identified by id,
// persists the result to disk, and returns the updated session.
func (f *FileSessionStore) UpdateSession(_ context.Context, id string, meta schema.SessionMeta) (*schema.Session, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	s, err := f.read(id)
	if err != nil {
		return nil, err
	}

	if err := mergeSessionMeta(s, meta); err != nil {
		return nil, err
	}
	if err := f.write(s); err != nil {
		return nil, err
	}
	return s, nil
}
