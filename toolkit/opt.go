package toolkit

import (
	// Packages
	client "github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
	env "github.com/mutablelogic/go-llm/auth/credentials/env"
	broadcaster "github.com/mutablelogic/go-llm/broadcaster"
	trace "go.opentelemetry.io/otel/trace"
)

////////////////////////////////////////////////////////////////////////////////
// TYPES

// Opt is a functional option for toolkit configuration.
type Opt func(*opt) error

type opt struct {
	tracer      trace.Tracer
	clientopts  []client.ClientOpt
	credentials CredentialsProvider
	broadcaster *broadcaster.Broadcaster
}

////////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

func (o *opt) apply(opt []Opt) error {
	o.defaults()

	// Apply options
	for _, fn := range opt {
		if err := fn(o); err != nil {
			return err
		}
	}

	// Return success
	return nil
}

func (o *opt) defaults() {
	o.tracer = nil
	o.clientopts = nil
	o.credentials = env.New()
	o.broadcaster = nil
}

////////////////////////////////////////////////////////////////////////////////
// OPTIONS

// WithBroadcaster sets the broadcaster which the toolkit publishes the
// changes to its tools to, as they're added, and as a namespace's tools are
// created, updated or deleted, whose table is ToolTable, and whose rows are
// the tools' canonical names
func WithBroadcaster(b *broadcaster.Broadcaster) Opt {
	return func(o *opt) error {
		if b == nil {
			return llm.ErrBadParameter.With("missing broadcaster")
		}
		o.broadcaster = b
		return nil
	}
}

// WithTracer sets the tracer used for tracing operations.
func WithTracer(tracer trace.Tracer) Opt {
	return func(o *opt) error {
		o.tracer = tracer
		return nil
	}
}

// WithClientOpts sets the client options for the HTTP clients of tools which
// call APIs, e.g. for timeouts.
func WithClientOpts(clientopts ...client.ClientOpt) Opt {
	return func(o *opt) error {
		o.clientopts = clientopts
		return nil
	}
}

// WithCredentialsProvider sets the provider of credentials, such as API
// keys, for tools which need them, which are environment variables, without
// one, or with a nil provider
func WithCredentialsProvider(provider CredentialsProvider) Opt {
	return func(o *opt) error {
		if provider == nil {
			o.credentials = env.New()
		} else {
			o.credentials = provider
		}
		return nil
	}
}
