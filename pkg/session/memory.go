package session

import (
	"context"
	"sort"
	"sync"
	"time"

	// Packages
	uuid "github.com/google/uuid"
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// MemoryStore is an in-memory implementation of Store.
// It is safe for concurrent use.
type MemoryStore struct {
	mu       sync.RWMutex
	sessions map[string]*schema.Session
}

var _ schema.Store = (*MemoryStore)(nil)

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// NewMemoryStore creates a new empty in-memory session store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		sessions: make(map[string]*schema.Session),
	}
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Create creates a new session with a unique ID and returns it.
func (m *MemoryStore) Create(_ context.Context, meta schema.SessionMeta) (*schema.Session, error) {
	if meta.Model == "" {
		return nil, llm.ErrBadParameter.With("model name is required")
	}

	now := time.Now()
	s := &schema.Session{
		ID:          uuid.New().String(),
		SessionMeta: meta,
		Messages:    make(schema.Conversation, 0),
		Created:     now,
		Modified:    now,
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	m.sessions[s.ID] = s

	return s, nil
}

// Get retrieves a session by ID.
func (m *MemoryStore) Get(_ context.Context, id string) (*schema.Session, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	s, ok := m.sessions[id]
	if !ok {
		return nil, llm.ErrNotFound.Withf("session %q", id)
	}
	return s, nil
}

// List returns sessions, ordered by last modified time (most recent first),
// with pagination support.
func (m *MemoryStore) List(_ context.Context, req schema.ListSessionRequest) (*schema.ListSessionResponse, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*schema.Session, 0, len(m.sessions))
	for _, s := range m.sessions {
		result = append(result, s)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Modified.After(result[j].Modified)
	})

	// Paginate
	total := uint(len(result))
	start := req.Offset
	if start > total {
		start = total
	}
	end := start + types.Value(req.Limit)
	if req.Limit == nil || end > total {
		end = total
	}

	return &schema.ListSessionResponse{
		Count:  total,
		Offset: req.Offset,
		Limit:  req.Limit,
		Body:   result[start:end],
	}, nil
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
func (m *MemoryStore) Write(_ *schema.Session) error {
	return nil
}

// Update applies non-zero fields from meta to the session identified by id.
func (m *MemoryStore) Update(_ context.Context, id string, meta schema.SessionMeta) (*schema.Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	s, ok := m.sessions[id]
	if !ok {
		return nil, llm.ErrNotFound.Withf("session %q", id)
	}

	if meta.Name != "" {
		s.Name = meta.Name
	}
	if meta.Model != "" {
		s.Model = meta.Model
	}
	if meta.Provider != "" {
		s.Provider = meta.Provider
	}
	if meta.SystemPrompt != "" {
		s.SystemPrompt = meta.SystemPrompt
	}
	if meta.Format != nil {
		s.Format = meta.Format
	}
	if meta.Thinking != nil {
		s.Thinking = meta.Thinking
	}
	if meta.ThinkingBudget > 0 {
		s.ThinkingBudget = meta.ThinkingBudget
	}
	s.Modified = time.Now()

	return s, nil
}
