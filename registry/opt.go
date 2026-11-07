package registry

import (
	"slices"
	"time"

	// Packages
	client "github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
	broadcaster "github.com/mutablelogic/go-llm/broadcaster"
	toolkit "github.com/mutablelogic/go-llm/toolkit"
	trace "go.opentelemetry.io/otel/trace"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Opt is a functional option for the registry
type Opt func(*opt) error

type opt struct {
	tracer        trace.Tracer
	credentials   CredentialsProvider
	clientopts    []client.ClientOpt
	providerStore ProviderStore
	sessionStore  SessionStore
	agentStore    AgentStore
	eventStore    EventStore
	eventBuilder  EventBuilder
	maxEvents     uint
	toolkit       *toolkit.Toolkit
	broadcaster   *broadcaster.Broadcaster
	resources     Resources

	// The namespaces whose tools, in a session, have the times of its
	// messages told to its model, and the time zone they're told in
	timesFor  []string
	timesZone func() *time.Location
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
	o.providerStore = nil
	o.sessionStore = nil
	o.agentStore = nil
	o.eventStore = nil
	o.eventBuilder = nil
	o.maxEvents = DefaultMaxEvents
	o.toolkit = nil
	o.broadcaster = broadcaster.New()
	o.resources = nil
	o.timesFor = nil
	o.timesZone = nil
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

// WithProviderStore sets the store of the providers, which the registry's providers
// are kept the same as, both ways, as Run syncs them, such as a database,
// which other servers share, or a file, so they're kept when the server
// restarts
func WithProviderStore(store ProviderStore) Opt {
	return func(o *opt) error {
		if store == nil {
			return llm.ErrBadParameter.With("missing store")
		}
		o.providerStore = store
		return nil
	}
}

// WithSessionStore sets the store of the sessions, such as a database,
// which other servers share, or files, so they're kept when the server
// restarts
func WithSessionStore(store SessionStore) Opt {
	return func(o *opt) error {
		if store == nil {
			return llm.ErrBadParameter.With("missing session store")
		}
		o.sessionStore = store
		return nil
	}
}

// WithAgentStore sets the store of the agents, such as a database, which
// other servers share, or files, so they're kept when the server restarts
func WithAgentStore(store AgentStore) Opt {
	return func(o *opt) error {
		if store == nil {
			return llm.ErrBadParameter.With("missing agent store")
		}
		o.agentStore = store
		return nil
	}
}

// WithEventStore sets the store of the outcomes of the events which the
// registry runs, such as heartbeats which fire, so they can be seen, and
// delivered. Without one, an event's outcome is only in the session it ran
// in.
func WithEventStore(store EventStore) Opt {
	return func(o *opt) error {
		if store == nil {
			return llm.ErrBadParameter.With("missing event store")
		}
		o.eventStore = store
		return nil
	}
}

// WithEventBuilder sets the function which returns how an event, which
// TriggerEvent is given, is run, such as with what the system says of the
// person, and the tools it's run without. Without one, its input says only
// the time, in UTC, and it's run with every tool of the session it's for.
func WithEventBuilder(fn EventBuilder) Opt {
	return func(o *opt) error {
		if fn == nil {
			return llm.ErrBadParameter.With("missing event builder")
		}
		o.eventBuilder = fn
		return nil
	}
}

// WithResources sets the resources, which messages' attachments, and the
// audio, documents and images which are transcribed, recognized, or
// embedded, refer to by their URIs, which are resource://<hash>, whose
// content is read, as they're sent to a model, or none, when attachments
// can't refer to resources
func WithResources(resources Resources) Opt {
	return func(o *opt) error {
		if resources == nil {
			return llm.ErrBadParameter.With("missing resources")
		}
		o.resources = resources
		return nil
	}
}

// WithMaxEvents sets the most events, such as heartbeats which fire, and
// events which are triggered, which run at once, which is DefaultMaxEvents,
// unless it's set. Those which are run once these are, wait until one of
// them has, and an event which was triggered is scheduled until it runs.
// Returns ErrBadParameter when it's zero.
func WithMaxEvents(n uint) Opt {
	return func(o *opt) error {
		if n == 0 {
			return llm.ErrBadParameter.With("the most events which run at once must be at least one")
		}
		o.maxEvents = n
		return nil
	}
}

// WithToolkit sets the toolkit of the tools which the registry's models can
// call
func WithToolkit(tools *toolkit.Toolkit) Opt {
	return func(o *opt) error {
		if tools == nil {
			return llm.ErrBadParameter.With("missing toolkit")
		}
		o.toolkit = tools
		return nil
	}
}

// WithTimes tells a model when each of the person's messages was sent, at
// its start, such as (Saturday 2026-11-07T12:38+01:00), in the time zone
// the function returns, or UTC, when it's nil, or returns nil, when
// the model can call a tool of one of the namespaces, such as memory, which
// makes a chat an assistant's, so the model knows the time, and how long
// it's been since an earlier message, without its system prompt changing
func WithTimes(zone func() *time.Location, namespaces ...string) Opt {
	return func(o *opt) error {
		o.timesFor, o.timesZone = slices.Clone(namespaces), zone
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

// WithBroadcaster sets the broadcaster which the registry publishes its
// events to, such as one which others share, so a server's events are
// served together. Its owner runs it. Without it, the registry has its own,
// which isn't run, so its watches end only when they're cancelled.
func WithBroadcaster(b *broadcaster.Broadcaster) Opt {
	return func(o *opt) error {
		if b == nil {
			return llm.ErrBadParameter.With("missing broadcaster")
		}
		o.broadcaster = b
		return nil
	}
}
