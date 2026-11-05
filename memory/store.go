package memory

import (
	"context"
	"sync"

	// Packages
	schema "github.com/mutablelogic/go-llm/memory/schema"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// store keeps the details in memory, so they aren't kept when the process
// ends
type store struct {
	mu      sync.Mutex
	details schema.Details
}

var _ Store = (*store)(nil)

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// NewStore returns a store which keeps the details in memory, so they
// aren't kept when the process ends, such as for tests, and examples
func NewStore() Store {
	return &store{}
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

func (s *store) Details(context.Context) (schema.Details, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return clone(s.details), nil
}

func (s *store) PutDetails(_ context.Context, details schema.Details) error {
	if err := details.Validate(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.details = clone(details)
	return nil
}
