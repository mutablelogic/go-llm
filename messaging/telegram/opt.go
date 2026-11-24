package telegram

import (
	// Packages
	client "github.com/mutablelogic/go-client"
	trace "go.opentelemetry.io/otel/trace"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Opt is an option of the bot
type Opt func(*opt) error

type opt struct {
	// The options of go-client, which apply to the bot's requests
	clientopts []client.ClientOpt

	// The tracer of what the bot does, or nil
	tracer trace.Tracer
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
// PUBLIC METHODS

// WithClientOpts are options of go-client, such as client.OptUserAgent, or
// client.OptTracer, whose spans are of each of the bot's requests, or
// client.OptEndpoint, for tests, which apply to the bot's requests
func WithClientOpts(opts ...client.ClientOpt) Opt {
	return func(o *opt) error {
		o.clientopts = append(o.clientopts, opts...)
		return nil
	}
}

// WithTracer sets the tracer of what the bot does, such as sending a
// message, which can be more than one request, and waiting while it's rate
// limited, or none, when it's nil. The spans of its requests are
// client.OptTracer's, with WithClientOpts.
func WithTracer(tracer trace.Tracer) Opt {
	return func(o *opt) error {
		o.tracer = tracer
		return nil
	}
}
