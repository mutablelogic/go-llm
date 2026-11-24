package registry

import (
	// Packages
	llm "github.com/mutablelogic/go-llm"
	broadcaster "github.com/mutablelogic/go-server/broadcaster"
	trace "go.opentelemetry.io/otel/trace"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Opt is an option of the registry
type Opt func(*opt) error

type opt struct {
	broadcaster *broadcaster.Broadcaster
	tracer      trace.Tracer
	maxBody     int64
}

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

// DefaultMaxBody is the largest body of a call which is received, by
// default, which is larger than services send, such as GitHub's 25 MB
// limit, which few events are near
const DefaultMaxBody = 4 << 20

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

// WithBroadcaster sets the broadcaster which Run watches, which the store
// publishes its changes to, such as the SQLite store's, with its
// WithBroadcaster: the webhooks, and calls, which are created, updated and
// deleted, of store.WebhookTable, and store.CallTable
func WithBroadcaster(b *broadcaster.Broadcaster) Opt {
	return func(o *opt) error {
		if b == nil {
			return llm.ErrBadParameter.With("missing broadcaster")
		}
		o.broadcaster = b
		return nil
	}
}

// WithTracer sets the tracer which traces the registry's methods, or none,
// when it's nil
func WithTracer(tracer trace.Tracer) Opt {
	return func(o *opt) error {
		o.tracer = tracer
		return nil
	}
}

// WithMaxBody sets the largest body of a call which is received, in bytes,
// beyond which it isn't
func WithMaxBody(n int64) Opt {
	return func(o *opt) error {
		if n <= 0 {
			return llm.ErrBadParameter.With("the largest body of a call is more than zero bytes")
		}
		o.maxBody = n
		return nil
	}
}
