package session

import (
	"context"
	"sort"
	"sync"
	"time"

	// Packages
	uuid "github.com/google/uuid"
	llm "github.com/mutablelogic/go-llm"
	opt "github.com/mutablelogic/go-llm/pkg/opt"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// MemoryStore is an in-memory implementation of Store.
// It is safe for concurrent use.
type MemoryStore struct {
	mu       sync.RWMutex
	sessions map[string]*Session
}

var _ Store = (*MemoryStore)(nil)

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// NewMemoryStore creates a new empty in-memory session store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		sessions: make(map[string]*Session),
	}
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Create creates a new session with a unique ID and returns it.
func (m *MemoryStore) Create(_ context.Context, name string, model schema.Model) (*Session, error) {
	if model.Name == "" {
		return nil, llm.ErrBadParameter.With("model name is required")
	}

	now := time.Now()
	s := &Session{
		ID:       uuid.New().String(),
		Name:     name,
		Model:    model,
		Messages: make(schema.Conversation, 0),
		Created:  now,
		Modified: now,
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	m.sessions[s.ID] = s

	return s, nil
}

// Get retrieves a session by ID.
func (m *MemoryStore) Get(_ context.Context, id string) (*Session, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	s, ok := m.sessions[id]
	if !ok {
		return nil, llm.ErrNotFound.Withf("session %q", id)
	}
	return s, nil
}

// List returns all sessions, ordered by last modified time (most recent first).
func (m *MemoryStore) List(_ context.Context, opts ...opt.Opt) ([]*Session, error) {
	o, err := opt.Apply(opts...)
	if err != nil {
		return nil, err
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*Session, 0, len(m.sessions))
	for _, s := range m.sessions {
		result = append(result, s)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Modified.After(result[j].Modified)
	})

	if limit := o.GetUint(opt.LimitKey); limit > 0 && int(limit) < len(result) {
		result = result[:limit]
	}

	return result, nil
}

// Delete removes a session by ID.
func (m *MemoryStore) Delete(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.sessions[id]; !ok {
		return llm.ErrNotFound.Withf("session %q", id)
	}
	delete(m.sessions, id)
	return nil
}

// Write is a no-op for the memory store since sessions are held
// as pointers in memory and mutations are visible immediately.
func (m *MemoryStore) Write(_ *Session) error {
	return nil
}
