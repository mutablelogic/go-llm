package registry

import (
	"context"
	"encoding/json"

	// Packages
	broadcaster "github.com/mutablelogic/go-llm/broadcaster"
	schema "github.com/mutablelogic/go-llm/broadcaster/schema"
)

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// ProviderTable is the table of the events of providers, whose rows are
	// their names in the registry
	ProviderTable = "provider"

	// SessionTable is the table of the events of sessions, whose rows are
	// their identifiers
	SessionTable = "session"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Broadcaster returns the broadcaster which the registry publishes its
// events to, so its events can be served, and it can be run
func (r *Registry) Broadcaster() *broadcaster.Broadcaster {
	return r.broadcaster
}

// Watch sends the registry's broadcaster's events to the channel, as the
// broadcaster's Watch does. The registry's events are of providers: when one
// is created, or deleted, and updated, when whether it can be reached
// changes, or its models do, and of sessions: when one is created, deleted,
// or updated, as its settings change, messages are added to it, or its
// status changes. A broadcaster which others share sends their events too.
func (r *Registry) Watch(ctx context.Context, ch chan<- schema.Event) (<-chan error, error) {
	return r.broadcaster.Watch(ctx, ch)
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// publish publishes the event of the row of the table, with the data, when
// there is any, which is JSON. It's called under the registry's lock, for
// providers, so the events are in the order of the changes, and never waits,
// so the lock isn't held for long.
func (r *Registry) publish(action schema.EventAction, table, row string, data any) {
	event := schema.Event{Action: action, Table: table, Row: row}
	if data != nil {
		if raw, err := json.Marshal(data); err == nil {
			event.Data = raw
		}
	}
	r.broadcaster.Publish(event)
}

// created publishes that the provider with the name was created
func (r *Registry) created(name string) {
	r.publish(schema.EventCreate, ProviderTable, name, nil)
}

// replaced publishes that the provider with the name was updated, as it
// was replaced
func (r *Registry) replaced(name string) {
	r.publish(schema.EventUpdate, ProviderTable, name, nil)
}

// deleted publishes that the provider with the name was deleted
func (r *Registry) deleted(name string) {
	r.publish(schema.EventDelete, ProviderTable, name, nil)
}

// updated publishes that the provider with the name was updated, when it's
// the cached provider, so a provider which was removed, or replaced, while
// it was pinged, or its models listed, isn't. It's checked and published
// under the registry's lock, so it can't be removed, or replaced, between.
func (r *Registry) updated(name string, c *cached) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.providers[name] == c {
		r.publish(schema.EventUpdate, ProviderTable, name, nil)
	}
}
