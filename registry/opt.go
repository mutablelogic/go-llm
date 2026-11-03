package registry

import (
	"slices"

	// Packages
	client "github.com/mutablelogic/go-client"
	trace "go.opentelemetry.io/otel/trace"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Opt is a functional option for the registry
type Opt func(*opt) error

type opt struct {
	tracer      trace.Tracer
	credentials CredentialsProvider
	clientopts  []client.ClientOpt
}

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

func (o *opt) apply(opts []Opt) error {
	o.defaults()

	// Apply options
	for _, fn := range opts {
		if err := fn(o); err != nil {
			return err
		}
	}

	// Return success
	return nil
}

func (o *opt) defaults() {
	o.tracer = nil
	o.credentials = envCredentials{}
	o.clientopts = nil
}

///////////////////////////////////////////////////////////////////////////////
// OPTIONS

// WithTracer sets the tracer, which traces the registry's operations, such
// as listing models, and generating messages
func WithTracer(tracer trace.Tracer) Opt {
	return func(o *opt) error {
		o.tracer = tracer
		return nil
	}
}

// WithClientOpts sets the options of the HTTP clients of the providers
// which the registry creates, such as their tracer, user agent and timeout
func WithClientOpts(opts ...client.ClientOpt) Opt {
	return func(o *opt) error {
		o.clientopts = slices.Clone(opts)
		return nil
	}
}

// WithCredentialsProvider sets the provider which expands credentials, such
// as the API key of a provider which is created, whose placeholders, such
// as ${OPENAI_KEY}, are replaced with the credentials they name. Without
// one, or with nil, the credentials are environment variables.
func WithCredentialsProvider(credentials CredentialsProvider) Opt {
	return func(o *opt) error {
		if credentials == nil {
			o.credentials = envCredentials{}
		} else {
			o.credentials = credentials
		}
		return nil
	}
}
