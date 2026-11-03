package registry

import (
	"context"
	"sync"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/provider/schema"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// watchers are the channels which the registry's events are sent to, each
// with a channel which is closed when the registry stops running, and
// whether it has stopped, so no more are watching
type watchers struct {
	sync.Mutex
	stopped  bool
	channels map[chan<- schema.Event]chan struct{}
}

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

// ProviderTable is the table of the events of providers, whose rows are
// their names in the registry
const ProviderTable = "provider"

// errStopped ends a watch, as the registry has stopped running
var errStopped = llm.ErrServiceUnavailable.With("registry stopped")

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Watch sends the registry's events to the channel, from when it returns, as
// it's watching before it does, until the context is cancelled, or the
// registry stops running, as there will be no more events. The events are of
// providers: when one is created, or deleted, and updated, when whether it
// can be reached changes, or its models do. It returns a channel which is
// sent nil when the context is cancelled, or ErrServiceUnavailable when the
// registry stops running, once the watch has ended, after which no more
// events are sent to the channel, so it can be closed. A registry which has
// stopped running isn't watched, and returns ErrServiceUnavailable, until it
// runs again. An event isn't waited for the channel to receive, so a channel
// which is full misses it, and one which needs every event should be
// buffered. Returns ErrBadParameter when there's no channel, and ErrConflict
// when the channel is watching already.
func (r *Registry) Watch(ctx context.Context, ch chan<- schema.Event) (<-chan error, error) {
	if ch == nil {
		return nil, llm.ErrBadParameter.With("missing channel")
	}

	// Add the channel, which is watching once at a time
	stop := make(chan struct{})
	r.watchers.Lock()
	if r.watchers.stopped {
		r.watchers.Unlock()
		return nil, errStopped
	} else if _, exists := r.watchers.channels[ch]; exists {
		r.watchers.Unlock()
		return nil, llm.ErrConflict.With("channel is watching already")
	} else if r.watchers.channels == nil {
		r.watchers.channels = make(map[chan<- schema.Event]chan struct{})
	}
	r.watchers.channels[ch] = stop
	r.watchers.Unlock()

	// The watch ends when the context is cancelled, or the registry stops.
	// It's removed before that's sent, under the lock events are sent under,
	// so once it's received, no more events are sent to the channel.
	done := make(chan error, 1)
	go func() {
		var err error
		select {
		case <-ctx.Done():
		case <-stop:
			err = errStopped
		}
		r.watchers.Lock()
		if r.watchers.channels[ch] == stop {
			delete(r.watchers.channels, ch)
		}
		r.watchers.Unlock()
		done <- err
	}()
	return done, nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// publish sends the event of the provider with the name to each channel
// which is watching, which can receive it now. It's called under the
// registry's lock, so the events are in the order of the changes, and never
// waits, so the lock isn't held for long.
func (r *Registry) publish(action schema.EventAction, name string) {
	event := schema.Event{Action: action, Table: ProviderTable, Row: name}
	r.watchers.Lock()
	defer r.watchers.Unlock()
	for ch := range r.watchers.channels {
		select {
		case ch <- event:
		default:
		}
	}
}

// created publishes that the provider with the name was created
func (r *Registry) created(name string) {
	r.publish(schema.EventCreate, name)
}

// replaced publishes that the provider with the name was updated, as it
// was replaced
func (r *Registry) replaced(name string) {
	r.publish(schema.EventUpdate, name)
}

// deleted publishes that the provider with the name was deleted
func (r *Registry) deleted(name string) {
	r.publish(schema.EventDelete, name)
}

// updated publishes that the provider with the name was updated, when it's
// the cached provider, so a provider which was removed, or replaced, while
// it was pinged, or its models listed, isn't. It's checked and published
// under the registry's lock, so it can't be removed, or replaced, between.
func (r *Registry) updated(name string, c *cached) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.providers[name] == c {
		r.publish(schema.EventUpdate, name)
	}
}

// startWatching lets channels watch the registry, as it's running
func (r *Registry) startWatching() {
	r.watchers.Lock()
	defer r.watchers.Unlock()
	r.watchers.stopped = false
}

// stopWatching ends every watch, as the registry has stopped running, and
// those made later, until it runs again
func (r *Registry) stopWatching() {
	r.watchers.Lock()
	defer r.watchers.Unlock()
	r.watchers.stopped = true
	for ch, stop := range r.watchers.channels {
		close(stop)
		delete(r.watchers.channels, ch)
	}
}
