package schema

import (
	"context"
)

///////////////////////////////////////////////////////////////////////////////
// INTERFACES

// Store is the interface for a heartbeat store, providing CRUD operations
// and scheduling queries for heartbeat records.
type Store interface {
	// Create persists a new heartbeat with the given metadata and returns it.
	Create(ctx context.Context, meta HeartbeatMeta) (*Heartbeat, error)

	// Get retrieves a heartbeat by its unique identifier.
	Get(ctx context.Context, id string) (*Heartbeat, error)

	// Delete removes a heartbeat by its unique identifier and returns it.
	Delete(ctx context.Context, id string) (*Heartbeat, error)

	// List returns all heartbeats. If includeFired is true, includes heartbeats
	// that have already fired; otherwise only pending heartbeats are returned.
	List(ctx context.Context, includeFired bool) ([]*Heartbeat, error)

	// Update modifies an existing heartbeat's metadata and returns the updated record.
	Update(ctx context.Context, id string, meta HeartbeatMeta) (*Heartbeat, error)

	// Next returns heartbeats that are due to fire (i.e., their scheduled time
	// has arrived or passed and they have not yet fired).
	Next(ctx context.Context) ([]*Heartbeat, error)
}
