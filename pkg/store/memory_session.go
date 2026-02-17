package store

import (
	"context"
	"sort"
	"sync"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// MemorySessionStore is an in-memory implementation of Store.
// It is safe for concurrent use.
type MemorySessionStore struct {
	mu       sync.RWMutex
	sessions map[string]*schema.Session
}

var _ schema.SessionStore = (*MemorySessionStore)(nil)

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// NewMemorySessionStore creates a new empty in-memory session store.
func NewMemorySessionStore() *MemorySessionStore {
	return &MemorySessionStore{
		sessions: make(map[string]*schema.Session),
	}
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// CreateSession creates a new session with a unique ID and returns it.
func (m *MemorySessionStore) CreateSession(_ context.Context, meta schema.SessionMeta) (*schema.Session, error) {
	s, err := newSession(meta)
	if err != nil {
		return nil, err
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	m.sessions[s.ID] = s

	return s, nil
}

// GetSession retrieves a session by ID.
func (m *MemorySessionStore) GetSession(_ context.Context, id string) (*schema.Session, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	s, ok := m.sessions[id]
	if !ok {
		return nil, llm.ErrNotFound.Withf("session %q", id)
	}
	return s, nil
}

// ListSessions returns sessions, ordered by last modified time (most recent first),
// with pagination support.
func (m *MemorySessionStore) ListSessions(_ context.Context, req schema.ListSessionRequest) (*schema.ListSessionResponse, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*schema.Session, 0, len(m.sessions))
	for _, s := range m.sessions {
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

// DeleteSession removes a session by ID.
func (m *MemorySessionStore) DeleteSession(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.sessions[id]; !ok {
		return llm.ErrNotFound.Withf("session %q", id)
	}
	delete(m.sessions, id)
	return nil
}

// WriteSession is a no-op for the memory store since sessions are held
// as pointers in memory and mutations are visible immediately.
func (m *MemorySessionStore) WriteSession(_ *schema.Session) error {
	return nil
}

// UpdateSession applies non-zero fields from meta to the session identified by id.
func (m *MemorySessionStore) UpdateSession(_ context.Context, id string, meta schema.SessionMeta) (*schema.Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	s, ok := m.sessions[id]
	if !ok {
		return nil, llm.ErrNotFound.Withf("session %q", id)
	}

	if err := mergeSessionMeta(s, meta); err != nil {
		return nil, err
	}
	return s, nil
}
