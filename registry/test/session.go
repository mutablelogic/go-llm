package test

import (
	"context"
	"encoding/json"
	"slices"
	"sync"
	"time"
	"uuid"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	eventschema "github.com/mutablelogic/go-llm/broadcaster/schema"
	registry "github.com/mutablelogic/go-llm/registry"
	schema "github.com/mutablelogic/go-llm/registry/schema"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// SessionStore is a store of sessions in memory, whose identifiers are
// version 7 UUIDs, which are in the order the sessions are created, for
// tests. Its watchers are sent the events which Changed is called with, as
// changes which others made.
type SessionStore struct {
	mu       sync.Mutex
	sessions map[uuid.UUID]schema.Session
	watchers map[chan<- eventschema.Event]struct{}
}

var _ registry.SessionStore = (*SessionStore)(nil)

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// NewSessionStore returns a store of sessions in memory, which has none
func NewSessionStore() *SessionStore {
	return &SessionStore{sessions: make(map[uuid.UUID]schema.Session), watchers: make(map[chan<- eventschema.Event]struct{})}
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

func (m *SessionStore) Create(_ context.Context, meta schema.SessionMeta) (schema.Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	session := schema.Session{ID: uuid.NewV7(), Serial: 1, Created: now, Modified: now, SessionMeta: meta}
	m.sessions[session.ID] = clone(session)
	return clone(session), nil
}

func (m *SessionStore) Get(_ context.Context, id uuid.UUID) (schema.Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	session, exists := m.sessions[id]
	if !exists {
		return schema.Session{}, llm.ErrNotFound.Withf("session %q", id)
	}
	return clone(session), nil
}

func (m *SessionStore) List(_ context.Context, req schema.SessionListRequest) (schema.SessionList, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	body := make([]schema.Session, 0, len(m.sessions))
	for _, session := range m.sessions {
		session = clone(session)
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
	m.sessions[session.ID] = clone(session)
	return clone(session), nil
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

// Watch sends the events which Changed is called with to the channel,
// until the context is cancelled, when the returned channel is sent nil
func (m *SessionStore) Watch(ctx context.Context, ch chan<- eventschema.Event) (<-chan error, error) {
	if ch == nil {
		return nil, llm.ErrBadParameter.With("missing channel")
	}
	m.mu.Lock()
	m.watchers[ch] = struct{}{}
	m.mu.Unlock()
	done := make(chan error, 1)
	go func() {
		<-ctx.Done()
		m.mu.Lock()
		delete(m.watchers, ch)
		m.mu.Unlock()
		done <- nil
	}()
	return done, nil
}

// Changed sends the event to the watchers, as a change which others made,
// such as another server, and returns how many it was sent to
func (m *SessionStore) Changed(event eventschema.Event) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	for ch := range m.watchers {
		ch <- event
	}
	return len(m.watchers)
}

// Len returns how many sessions there are
func (m *SessionStore) Len() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.sessions)
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// clone returns a copy of the session, which shares none of its pointers,
// maps or slices, so a session which is stored, or returned, can't be
// changed but by Put
func clone(session schema.Session) schema.Session {
	data, err := json.Marshal(session)
	if err != nil {
		panic(err)
	}
	var result schema.Session
	if err := json.Unmarshal(data, &result); err != nil {
		panic(err)
	}
	return result
}
