package heartbeat

import (
	"context"
	"time"
	"uuid"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	broadcaster "github.com/mutablelogic/go-llm/broadcaster"
	schema "github.com/mutablelogic/go-llm/heartbeat/schema"
	trace "go.opentelemetry.io/otel/trace"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Opt is an option of the heartbeats
type Opt func(*opt) error

type opt struct {
	broadcaster *broadcaster.Broadcaster
	tracer      trace.Tracer
	zone        func() *time.Location
	session     SessionFunc
	retention   time.Duration
	run         RunFunc
}

// RunFunc runs a heartbeat which fired, such as by asking a model its prompt,
// in its session, which is called in a goroutine of its own, so a run which
// takes a while doesn't hold up the others. What it returns is logged.
type RunFunc func(context.Context, schema.FireEvent) error

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

// WithBroadcaster sets the broadcaster which the heartbeats publish their
// changes to, including when one fires, as when it fired is changed, whose
// schema is Name, whose table is HeartbeatTable, and whose rows are their
// identifiers. Run watches it for the sessions which are deleted, whose
// heartbeats are deleted with them.
func WithBroadcaster(b *broadcaster.Broadcaster) Opt {
	return func(o *opt) error {
		if b == nil {
			return llm.ErrBadParameter.With("missing broadcaster")
		}
		o.broadcaster = b
		return nil
	}
}

// WithTracer sets the tracer which traces the heartbeats' changes, or none,
// when it's nil
func WithTracer(tracer trace.Tracer) Opt {
	return func(o *opt) error {
		o.tracer = tracer
		return nil
	}
}

// WithRun sets the function which runs each heartbeat which fires, once, on
// the server which fires it, or none, so heartbeats fire, and nothing runs
func WithRun(fn RunFunc) Opt {
	return func(o *opt) error {
		o.run = fn
		return nil
	}
}

// WithSessions sets the function which says whether a session exists, so a
// heartbeat is only created for one which does, and one whose session was
// deleted is deleted, rather than fired, such as when the event that it was
// deleted was missed
func WithSessions(fn SessionFunc) Opt {
	return func(o *opt) error {
		o.session = fn
		return nil
	}
}

// WithRetention sets how long a heartbeat which won't fire again, such as a
// one-off which fired, is kept, after it last fired, or was changed, before
// it's deleted, which is a week, unless it's set. Returns ErrBadParameter
// when it isn't positive.
func WithRetention(d time.Duration) Opt {
	return func(o *opt) error {
		if d <= 0 {
			return llm.ErrBadParameter.With("retention must be positive")
		}
		o.retention = d
		return nil
	}
}

// WithZone sets the function which returns the person's time zone, or nil,
// when it isn't known, which the schedules without a time zone of their own
// are in, so they follow the person, as it changes, such as the memory's
func WithZone(fn func() *time.Location) Opt {
	return func(o *opt) error {
		o.zone = fn
		return nil
	}
}
