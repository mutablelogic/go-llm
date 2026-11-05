package heartbeat

import (
	"context"
	"sync"
	"uuid"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/heartbeat/schema"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Store keeps the heartbeats, such as in files, or a database, so they're
// kept when the server restarts. A store which servers share changes a
// heartbeat at once, with Update, so only one of them fires it.
type Store interface {
	// Create adds the heartbeat, or returns ErrConflict when there's one
	// with its identifier
	Create(context.Context, schema.Heartbeat) error

	// Get returns the heartbeat with the identifier, or ErrNotFound when
	// there's none
	Get(context.Context, uuid.UUID) (schema.Heartbeat, error)

	// List returns the heartbeats, in any order
	List(context.Context) ([]schema.Heartbeat, error)

	// Update calls the function with the heartbeat with the identifier, which
	// it changes, and replaces it with what the function changed it to, and
	// returns it, so no other change, such as by another server, is made
	// between them, or returns the function's error, without changing it, or
	// ErrNotFound when there's none
	Update(context.Context, uuid.UUID, func(*schema.Heartbeat) error) (schema.Heartbeat, error)

	// Delete deletes the heartbeat with the identifier, and returns it, as it
	// was, or ErrNotFound when there's none
	Delete(context.Context, uuid.UUID) (schema.Heartbeat, error)
}

// store keeps the heartbeats in memory, so they aren't kept when the
// process ends
type store struct {
	mu         sync.Mutex
	heartbeats map[uuid.UUID]schema.Heartbeat
}

var _ Store = (*store)(nil)

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// NewStore returns a store which keeps the heartbeats in memory, so they
// aren't kept when the process ends, such as for tests, and examples
func NewStore() Store {
	return &store{heartbeats: make(map[uuid.UUID]schema.Heartbeat)}
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

func (s *store) Create(_ context.Context, heartbeat schema.Heartbeat) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.heartbeats[heartbeat.ID]; exists {
		return llm.ErrConflict.Withf("heartbeat %q exists", heartbeat.ID)
	}
	s.heartbeats[heartbeat.ID] = Clone(heartbeat)
	return nil
}

func (s *store) Get(_ context.Context, id uuid.UUID) (schema.Heartbeat, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if heartbeat, ok := s.heartbeats[id]; ok {
		return Clone(heartbeat), nil
	}
	return schema.Heartbeat{}, llm.ErrNotFound.Withf("heartbeat %q", id)
}

func (s *store) List(context.Context) ([]schema.Heartbeat, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := make([]schema.Heartbeat, 0, len(s.heartbeats))
	for _, heartbeat := range s.heartbeats {
		result = append(result, Clone(heartbeat))
	}
	return result, nil
}

func (s *store) Update(_ context.Context, id uuid.UUID, fn func(*schema.Heartbeat) error) (schema.Heartbeat, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	heartbeat, ok := s.heartbeats[id]
	if !ok {
		return schema.Heartbeat{}, llm.ErrNotFound.Withf("heartbeat %q", id)
	}
	heartbeat = Clone(heartbeat)
	if err := fn(&heartbeat); err != nil {
		return schema.Heartbeat{}, err
	}
	heartbeat.ID = id
	s.heartbeats[id] = Clone(heartbeat)
	return heartbeat, nil
}

func (s *store) Delete(_ context.Context, id uuid.UUID) (schema.Heartbeat, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	heartbeat, ok := s.heartbeats[id]
	if !ok {
		return schema.Heartbeat{}, llm.ErrNotFound.Withf("heartbeat %q", id)
	}
	delete(s.heartbeats, id)
	return heartbeat, nil
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC FUNCTIONS

// Clone returns a copy of the heartbeat, whose times are copies, so changing
// it doesn't change the heartbeat
func Clone(heartbeat schema.Heartbeat) schema.Heartbeat {
	if heartbeat.At != nil {
		at := *heartbeat.At
		heartbeat.At = &at
	}
	if heartbeat.Fired != nil {
		fired := *heartbeat.Fired
		heartbeat.Fired = &fired
	}
	if heartbeat.Next != nil {
		next := *heartbeat.Next
		heartbeat.Next = &next
	}
	return heartbeat
}
