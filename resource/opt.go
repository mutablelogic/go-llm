package resource

import (
	"context"
	"time"
	"uuid"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	broadcaster "github.com/mutablelogic/go-llm/broadcaster"
	trace "go.opentelemetry.io/otel/trace"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Opt is an option of the resources
type Opt func(*opt) error

type opt struct {
	broadcaster *broadcaster.Broadcaster
	tracer      trace.Tracer
	session     SessionFunc
	retention   time.Duration
	maxSize     int64
}

// SessionFunc returns nil when the session with the identifier exists, or
// ErrNotFound when it doesn't
type SessionFunc func(context.Context, uuid.UUID) error

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

func (o *opt) apply(opts []Opt) error {
	for _, fn := range opts {
		if err := fn(o); err != nil {
			return err
		}
	}
	return nil
}

///////////////////////////////////////////////////////////////////////////////
// OPTIONS

// WithBroadcaster sets the broadcaster which the resources publish their
// changes to, whose schema is schema.Name, whose table is
// schema.ResourceTable, and whose rows are their hashes, with their
// sessions. Run watches it for the sessions which are deleted, whose
// resources are deleted with them.
func WithBroadcaster(b *broadcaster.Broadcaster) Opt {
	return func(o *opt) error {
		if b == nil {
			return llm.ErrBadParameter.With("missing broadcaster")
		}
		o.broadcaster = b
		return nil
	}
}

// WithTracer sets the tracer which traces the resources' changes, or none,
// when it's nil
func WithTracer(tracer trace.Tracer) Opt {
	return func(o *opt) error {
		o.tracer = tracer
		return nil
	}
}

// WithSessions sets the function which says whether a session exists, so a
// resource is only created in one which does, and one whose session was
// deleted is deleted, such as when the event that it was deleted was missed
func WithSessions(fn SessionFunc) Opt {
	return func(o *opt) error {
		o.session = fn
		return nil
	}
}

// WithRetention sets how long a resource on its own, without a session, is
// kept, after it was last used, before it's deleted, which is a day, unless
// it's set. Returns ErrBadParameter when it isn't positive.
func WithRetention(d time.Duration) Opt {
	return func(o *opt) error {
		if d <= 0 {
			return llm.ErrBadParameter.With("retention must be positive")
		}
		o.retention = d
		return nil
	}
}

// WithMaxSize sets the largest content of a resource, in bytes, which is
// schema.DefaultMaxSize, unless it's set. Returns ErrBadParameter when it
// isn't positive.
func WithMaxSize(size int64) Opt {
	return func(o *opt) error {
		if size <= 0 {
			return llm.ErrBadParameter.With("max size must be positive")
		}
		o.maxSize = size
		return nil
	}
}
