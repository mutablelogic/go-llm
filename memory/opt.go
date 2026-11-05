package memory

import (
	// Packages
	llm "github.com/mutablelogic/go-llm"
	broadcaster "github.com/mutablelogic/go-llm/broadcaster"
	trace "go.opentelemetry.io/otel/trace"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Opt is an option of the memory
type Opt func(*opt) error

type opt struct {
	broadcaster *broadcaster.Broadcaster
	tracer      trace.Tracer
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

// WithBroadcaster sets the broadcaster which the memory publishes its
// changes to: when a detail becomes known, or declined, which is created,
// changes, which is updated, or is forgotten, which is deleted. Their schema
// is Name, which is memory, their table is DetailTable, their rows are the
// details' fields, such as location, and their data is the detail, when
// there is one.
func WithBroadcaster(b *broadcaster.Broadcaster) Opt {
	return func(o *opt) error {
		if b == nil {
			return llm.ErrBadParameter.With("missing broadcaster")
		}
		o.broadcaster = b
		return nil
	}
}

// WithTracer sets the tracer which traces the memory's changes, and
// loading its details, or none, when it's nil
func WithTracer(tracer trace.Tracer) Opt {
	return func(o *opt) error {
		o.tracer = tracer
		return nil
	}
}
