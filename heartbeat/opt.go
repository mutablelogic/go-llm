package heartbeat

import (
	"time"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	broadcaster "github.com/mutablelogic/go-llm/broadcaster"
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
}

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
// changes to, whose schema is Name, whose table is HeartbeatTable, and whose
// rows are their identifiers, and that they fire, whose table is FireTable,
// and whose data is what fired. Run watches it for the sessions which are
// deleted, whose heartbeats are deleted with them.
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

// WithZone sets the function which returns the person's time zone, or nil,
// when it isn't known, which the schedules without a time zone of their own
// are in, so they follow the person, as it changes, such as the memory's
func WithZone(fn func() *time.Location) Opt {
	return func(o *opt) error {
		o.zone = fn
		return nil
	}
}
