// Package webhook is the namespace of the webhooks' tools, of a webhook
// server, whose URLs external services call, and which is watched for their
// calls. It has no tools yet.
package webhook

import (
	"context"
	"strings"
	"sync"
	"time"

	// Packages
	client "github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
	toolkit "github.com/mutablelogic/go-llm/toolkit"
	toolschema "github.com/mutablelogic/go-llm/toolkit/schema"
	httpclient "github.com/mutablelogic/go-llm/webhook/httpclient"
	eventschema "github.com/mutablelogic/go-server/broadcaster/schema"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Namespace is the namespace of the webhooks' tools, of a webhook server
type Namespace struct {
	client *httpclient.Client

	// Held while it's run, which changes what's below
	running sync.Mutex

	// Whether a call was seen, and the serial number of the last which was
	// handled, or before the first, which those which were missed are after
	seen bool
	last uint64

	// When the latest call which was handled was received, or zero, before
	// any was
	latest time.Time

	// The last call which was handled as it was missed, since it started
	// watching, as it can be told of it too
	missed uint64
}

var _ toolkit.Namespace = (*Namespace)(nil)
var _ toolkit.Runner = (*Namespace)(nil)

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

// Name is the name of the namespace of the webhooks' tools
const Name = "webhook"

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// New returns the namespace of the webhook server's API, at the endpoint,
// such as http://localhost:8087/api, with the client options. The server
// isn't called until the namespace is run.
func New(endpoint string, opts ...client.ClientOpt) (*Namespace, error) {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return nil, llm.ErrBadParameter.With("missing webhook endpoint")
	}

	c, err := httpclient.New(endpoint, opts...)
	if err != nil {
		return nil, err
	}
	return &Namespace{client: c}, nil
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

func (*Namespace) Name() string { return Name }

// Type returns that the namespace's tools are built in
func (*Namespace) Type() string { return toolschema.NamespaceBuiltin }

// Description returns what the namespace's tools are for
func (*Namespace) Description() string {
	return "Webhooks, which are URLs which external services call, such as when an issue is opened"
}

// SystemPrompt returns no system prompt, as there are no tools yet
func (*Namespace) SystemPrompt() string { return "" }

// Tools returns the namespace's tools, of which there are none yet
func (*Namespace) Tools() []llm.Tool { return nil }

// Watch sends the namespace's tools, which don't change
func (ns *Namespace) Watch(ctx context.Context, ch chan<- eventschema.Change) error {
	return toolkit.WatchTools(ctx, ch, ns.Tools())
}
