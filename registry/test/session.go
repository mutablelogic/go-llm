package test

import (
	"context"
	"slices"
	"sync"
	"time"
	"uuid"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	registry "github.com/mutablelogic/go-llm/registry"
	schema "github.com/mutablelogic/go-llm/registry/schema"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// SessionStore is a store of sessions in memory, whose identifiers are
// version 7 UUIDs, which are in the order the sessions are created, for tests
type SessionStore struct {
	mu       sync.Mutex
	sessions map[uuid.UUID]schema.Session
}

var _ registry.SessionStore = (*SessionStore)(nil)

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// NewSessionStore returns a store of sessions in memory, which has none
func NewSessionStore() *SessionStore {
	return &SessionStore{sessions: make(map[uuid.UUID]schema.Session)}
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

func (m *SessionStore) Create(_ context.Context, meta schema.SessionMeta) (schema.Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	session := schema.Session{ID: uuid.NewV7(), Serial: 1, Created: now, Modified: now, SessionMeta: meta}
	m.sessions[session.ID] = session
	return session, nil
}

func (m *SessionStore) Get(_ context.Context, id uuid.UUID) (schema.Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	session, exists := m.sessions[id]
	if !exists {
		return schema.Session{}, llm.ErrNotFound.Withf("session %q", id)
	}
	return session, nil
}

func (m *SessionStore) List(_ context.Context, req schema.SessionListRequest) (schema.SessionList, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	body := make([]schema.Session, 0, len(m.sessions))
	for _, session := range m.sessions {
		session.Messages = nil
		body = append(body, session)
	}
	slices.SortFunc(body, func(a, b schema.Session) int {
		if c := b.Modified.Compare(a.Modified); c != 0 {
			return c
		}
		return b.ID.Compare(a.ID)
	})
	count := uint(len(body))
	body = body[min(req.Offset, count):]
	if req.Limit != nil {
		body = body[:min(*req.Limit, uint(len(body)))]
	}
	return schema.SessionList{SessionListRequest: req, Count: count, Body: body}, nil
}

func (m *SessionStore) Put(_ context.Context, session schema.Session) (schema.Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	existing, exists := m.sessions[session.ID]
	if !exists {
		return schema.Session{}, llm.ErrNotFound.Withf("session %q", session.ID)
	} else if existing.Serial != session.Serial {
		return schema.Session{}, llm.ErrConflict.Withf("session %q was changed", session.ID)
	}
	session.Serial++
	session.Created = existing.Created
	session.Modified = time.Now()
	m.sessions[session.ID] = session
	return session, nil
}

func (m *SessionStore) Delete(_ context.Context, id uuid.UUID, serial uint64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	existing, exists := m.sessions[id]
	if !exists {
		return nil
	} else if existing.Serial != serial {
		return llm.ErrConflict.Withf("session %q was changed", id)
	}
	delete(m.sessions, id)
	return nil
}

// Len returns how many sessions there are
func (m *SessionStore) Len() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.sessions)
}
