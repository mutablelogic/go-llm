// Package broadcaster publishes events, which are changes, such as to a
// registry's providers, or its sessions, to the channels which watch it, so
// a server's clients see the changes as they happen
package broadcaster

import (
	"context"
	"sync"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/broadcaster/schema"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Broadcaster sends the events which are published to each channel which is
// watching it. It's watched from when it's made, until it stops running,
// and again when it runs again. It's safe to use concurrently.
type Broadcaster struct {
	mu       sync.Mutex
	stopped  bool
	channels map[chan<- schema.Event]chan struct{}
}

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

// errStopped ends a watch, as the broadcaster has stopped running
var errStopped = llm.ErrServiceUnavailable.With("broadcaster stopped")

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// New returns a broadcaster, which can be watched
func New() *Broadcaster {
	return &Broadcaster{channels: make(map[chan<- schema.Event]chan struct{})}
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Run runs the broadcaster until the context is cancelled, when every watch
// ends, with ErrServiceUnavailable, as there will be no more events, such as
// when the server is stopping, and it isn't watched again until it runs
// again. It returns nil.
func (b *Broadcaster) Run(ctx context.Context) error {
	b.mu.Lock()
	b.stopped = false
	b.mu.Unlock()
	<-ctx.Done()
	b.stop()
	return nil
}

// Watch sends the events which are published to the channel, from when it
// returns, as it's watching before it does, until the context is cancelled,
// or the broadcaster stops running, as there will be no more events. It
// returns a channel which is sent nil when the context is cancelled, or
// ErrServiceUnavailable when the broadcaster stops running, once the watch
// has ended, after which no more events are sent to the channel, so it can
// be closed. A broadcaster which has stopped running isn't watched, and
// returns ErrServiceUnavailable, until it runs again. An event isn't waited
// for the channel to receive, so a channel which is full misses it, and one
// which needs every event should be buffered. Returns ErrBadParameter when
// there's no channel, and ErrConflict when the channel is watching already.
func (b *Broadcaster) Watch(ctx context.Context, ch chan<- schema.Event) (<-chan error, error) {
	if ch == nil {
		return nil, llm.ErrBadParameter.With("missing channel")
	}

	// Add the channel, which is watching once at a time
	stop := make(chan struct{})
	b.mu.Lock()
	if b.stopped {
		b.mu.Unlock()
		return nil, errStopped
	} else if _, exists := b.channels[ch]; exists {
		b.mu.Unlock()
		return nil, llm.ErrConflict.With("channel is watching already")
	} else if b.channels == nil {
		b.channels = make(map[chan<- schema.Event]chan struct{})
	}
	b.channels[ch] = stop
	b.mu.Unlock()

	// The watch ends when the context is cancelled, or the broadcaster
	// stops. It's removed before that's sent, under the lock events are sent
	// under, so once it's received, no more events are sent to the channel.
	done := make(chan error, 1)
	go func() {
		var err error
		select {
		case <-ctx.Done():
		case <-stop:
			err = errStopped
		}
		b.mu.Lock()
		if b.channels[ch] == stop {
			delete(b.channels, ch)
		}
		b.mu.Unlock()
		done <- err
	}()
	return done, nil
}

// Publish sends the event to each channel which is watching, which can
// receive it now. It never waits, so it can be called under a lock, which
// keeps the events in the order of the changes.
func (b *Broadcaster) Publish(event schema.Event) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for ch := range b.channels {
		select {
		case ch <- event:
		default:
		}
	}
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// stop ends every watch, as the broadcaster has stopped running, and those
// made later, until it runs again
func (b *Broadcaster) stop() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.stopped = true
	for ch, stop := range b.channels {
		close(stop)
		delete(b.channels, ch)
	}
}
