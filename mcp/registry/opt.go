package registry

import (
	"strings"

	// Packages
	client "github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
	broadcaster "github.com/mutablelogic/go-llm/broadcaster"
	mcpclient "github.com/mutablelogic/go-llm/mcp/client"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Opt is an option of the registry
type Opt func(*opt) error

type opt struct {
	store       Store
	broadcaster *broadcaster.Broadcaster
	clientopts  []mcpclient.Opt
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
	o.store = NewMemoryStore()
	o.broadcaster = nil
	o.clientopts = nil
}

///////////////////////////////////////////////////////////////////////////////
// OPTIONS

// WithStore is the store of the connectors, which keeps them, such as in a
// file, or a database, rather than in memory
func WithStore(store Store) Opt {
	return func(o *opt) error {
		if store == nil {
			return llm.ErrBadParameter.With("missing store")
		}
		o.store = store
		return nil
	}
}

// WithBroadcaster is the broadcaster the registry publishes its
// connectors' connections to, such as the server's, so its clients see
// them: a connector whose server is connected is created, with which
// server it is, and one whose server is disconnected is deleted, with why,
// when it wasn't disconnected
func WithBroadcaster(b *broadcaster.Broadcaster) Opt {
	return func(o *opt) error {
		if b == nil {
			return llm.ErrBadParameter.With("missing broadcaster")
		}
		o.broadcaster = b
		return nil
	}
}

// WithClientOpts are options of go-client, such as client.OptTrace, which
// apply to the requests of the clients of each connector
func WithClientOpts(opts ...client.ClientOpt) Opt {
	return func(o *opt) error {
		o.clientopts = append(o.clientopts, mcpclient.WithClientOpts(opts...))
		return nil
	}
}

// WithName is the name, and version, which each connector's MCP server is
// told, as a client connects to it, rather than schema.DefaultName, and
// schema.DefaultVersion. The version can be empty.
func WithName(name, version string) Opt {
	return func(o *opt) error {
		if strings.TrimSpace(name) == "" {
			return llm.ErrBadParameter.With("name is required")
		}
		o.clientopts = append(o.clientopts, mcpclient.WithName(name, version))
		return nil
	}
}
