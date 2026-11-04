//go:build !client

package main

import (
	"context"
	"encoding/json"
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

// sessionStore is a store of the registry's sessions in the command's
// defaults, as a list of them, with their messages, in order of their
// identifiers, under one key, so they're kept when the server restarts. As
// each change writes every session, it's for a few sessions, rather than
// many. It's safe to use concurrently.
type sessionStore struct {
	mu       sync.Mutex
	defaults defaults
}

var _ registry.SessionStore = (*sessionStore)(nil)

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

// sessionsKey is the key of the defaults which the sessions are kept in
const sessionsKey = "llm.sessions"

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// newSessionStore returns a store of the registry's sessions in the defaults
func newSessionStore(d defaults) (registry.SessionStore, error) {
	if d == nil {
		return nil, llm.ErrBadParameter.With("missing defaults")
	}
	return &sessionStore{defaults: d}, nil
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Create puts a session with the settings, and no messages, in the defaults,
// with a new identifier, which is a random UUID, and a serial of one, and
// returns it
func (s *sessionStore) Create(_ context.Context, meta schema.SessionMeta) (schema.Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	list, err := s.read()
	if err != nil {
		return schema.Session{}, err
	}
	now := time.Now().UTC()
	session := schema.Session{ID: uuid.New(), Serial: 1, Created: now, Modified: now, SessionMeta: meta}
	if err := s.write(append(list, session)); err != nil {
		return schema.Session{}, err
	}
	return session, nil
}

// Get returns the session with the identifier, with its messages. Returns
// ErrNotFound when there's none.
func (s *sessionStore) Get(_ context.Context, id uuid.UUID) (schema.Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	list, err := s.read()
	if err != nil {
		return schema.Session{}, err
	}
	i := slices.IndexFunc(list, func(session schema.Session) bool { return session.ID == id })
	if i < 0 {
		return schema.Session{}, llm.ErrNotFound.Withf("session %q", id)
	}
	return list[i], nil
}

// List returns the sessions, without their messages, most recently modified
// first, from the request's offset, and at most its limit of them, and how
// many there are
func (s *sessionStore) List(_ context.Context, req schema.SessionListRequest) (schema.SessionList, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	list, err := s.read()
	if err != nil {
		return schema.SessionList{}, err
	}
	for i := range list {
		list[i].Messages = nil
	}
	slices.SortFunc(list, func(a, b schema.Session) int {
		if c := b.Modified.Compare(a.Modified); c != 0 {
			return c
		}
		return a.ID.Compare(b.ID)
	})
	count := uint(len(list))
	list = list[min(req.Offset, count):]
	if req.Limit != nil {
		list = list[:min(*req.Limit, uint(len(list)))]
	}
	return schema.SessionList{SessionListRequest: req, Count: count, Body: list}, nil
}

// Put replaces the session with its identifier, when its serial is that
// session's, and returns it, with its serial one more, and when it was
// modified, keeping when it was created. Returns ErrNotFound when there's
// none, and ErrConflict when its serial isn't the session's.
func (s *sessionStore) Put(_ context.Context, session schema.Session) (schema.Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	list, err := s.read()
	if err != nil {
		return schema.Session{}, err
	}
	i := slices.IndexFunc(list, func(other schema.Session) bool { return other.ID == session.ID })
	if i < 0 {
		return schema.Session{}, llm.ErrNotFound.Withf("session %q", session.ID)
	} else if list[i].Serial != session.Serial {
		return schema.Session{}, llm.ErrConflict.Withf("session %q is at serial %d, not %d", session.ID, list[i].Serial, session.Serial)
	}
	session.Serial++
	session.Created = list[i].Created
	session.Modified = time.Now().UTC()
	list[i] = session
	if err := s.write(list); err != nil {
		return schema.Session{}, err
	}
	return session, nil
}

// Delete deletes the session with the identifier from the defaults, when the
// serial is its serial, or does nothing when there's none. Returns
// ErrConflict when the serial isn't its serial.
func (s *sessionStore) Delete(_ context.Context, id uuid.UUID, serial uint64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	list, err := s.read()
	if err != nil {
		return err
	}
	i := slices.IndexFunc(list, func(session schema.Session) bool { return session.ID == id })
	if i < 0 {
		return nil
	} else if list[i].Serial != serial {
		return llm.ErrConflict.Withf("session %q is at serial %d, not %d", id, list[i].Serial, serial)
	}
	return s.write(slices.Delete(list, i, i+1))
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// read returns the sessions in the defaults, or none when there are none.
// The defaults have the sessions as they were set, or as JSON was decoded
// into, when they were read from disk, so they're encoded as JSON, and
// decoded as sessions, which are then the caller's to change. The caller
// holds the lock.
func (s *sessionStore) read() ([]schema.Session, error) {
	value := s.defaults.Get(sessionsKey)
	if value == nil {
		return nil, nil
	}
	data, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	var list []schema.Session
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, llm.ErrInternalServerError.Withf("%s: %v", sessionsKey, err)
	}
	return list, nil
}

// write sets the sessions in the defaults, in order of their identifiers,
// or removes them, when there are none. The caller holds the lock.
func (s *sessionStore) write(list []schema.Session) error {
	if len(list) == 0 {
		return s.defaults.Set(sessionsKey, nil)
	}
	slices.SortFunc(list, func(a, b schema.Session) int { return a.ID.Compare(b.ID) })
	return s.defaults.Set(sessionsKey, list)
}
