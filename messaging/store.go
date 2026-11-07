package messaging

import (
	"context"
	"slices"
	"sync"

	// Packages
	schema "github.com/mutablelogic/go-llm/messaging/schema"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Store keeps the channels, such as in a file, so they're kept when the
// server restarts. There are few of them, so they're loaded, and saved, as
// a whole.
type Store interface {
	// Load returns the channels, or none, when none were saved
	Load(context.Context) ([]schema.Channel, error)

	// Save replaces the channels with these
	Save(context.Context, []schema.Channel) error
}

// store keeps the channels in memory, so they aren't kept when the process
// ends
type store struct {
	mu       sync.Mutex
	channels []schema.Channel
}

var _ Store = (*store)(nil)

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// NewStore returns a store which keeps the channels in memory, so they
// aren't kept when the process ends, such as for tests
func NewStore() Store {
	return &store{}
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

func (s *store) Load(context.Context) ([]schema.Channel, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.channels), nil
}

func (s *store) Save(_ context.Context, channels []schema.Channel) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.channels = slices.Clone(channels)
	return nil
}
