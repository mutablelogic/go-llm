package toolkit

import (
	// Packages
	client "github.com/mutablelogic/go-client"
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
	o.credentials = envCredentials{}
}

////////////////////////////////////////////////////////////////////////////////
// OPTIONS

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
			o.credentials = envCredentials{}
		} else {
			o.credentials = provider
		}
		return nil
	}
}
