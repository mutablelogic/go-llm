package registry

import (
	"slices"

	// Packages
	client "github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
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
	store       Store
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
	o.store = nil
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

// WithStore sets the store of the providers, which the registry's providers
// are kept the same as, both ways, as Run syncs them, such as a database,
// which other servers share, or a file, so they're kept when the server
// restarts
func WithStore(store Store) Opt {
	return func(o *opt) error {
		if store == nil {
			return llm.ErrBadParameter.With("missing store")
		}
		o.store = store
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
