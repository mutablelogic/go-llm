// Package aftership is the namespace of the AfterShip tools, which track
// shipments
package aftership

import (
	"context"
	"encoding/json"
	"slices"

	// Packages
	client "github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
	toolkit "github.com/mutablelogic/go-llm/toolkit"
	httpclient "github.com/mutablelogic/go-llm/toolkit/aftership/httpclient"
	toolschema "github.com/mutablelogic/go-llm/toolkit/schema"
	webhook "github.com/mutablelogic/go-llm/webhook/protocol/aftership"
	eventschema "github.com/mutablelogic/go-server/broadcaster/schema"
)

////////////////////////////////////////////////////////////////////////////////
// TYPES

// namespace is the namespace of the AfterShip tools
type namespace struct {
	tools []llm.Tool
}

var _ toolkit.Namespace = (*namespace)(nil)

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

// Name is the name of the namespace of the AfterShip tools
const Name = "aftership"

// systemPrompt is how a webhook is set up, with the webhook namespace's tools,
// and the person
const systemPrompt = `AfterShip can call a webhook when a shipment changes. None of the aftership tools set it up, and the person has to register it. When they want it, and you have webhook:create:

1. Create the webhook, with webhook:create, of type aftership.
2. Give the person its URL, and ask them to add it at ` + webhook.RegisterPage + `.
3. Ask them for the webhook secret which that page gives, and set it with webhook:update.

It's called for every shipment of the person's account, so a conversation only needs one. Look for this conversation's with webhook:list, before you create another, and ask the person whether they've registered one already.`

////////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// New returns the namespace of the AfterShip tools, which share a client with the API key
func New(apiKey string, opts ...client.ClientOpt) (toolkit.Namespace, error) {
	client, err := httpclient.New(apiKey, opts...)
	if err != nil {
		return nil, err
	}
	return &namespace{
		tools: []llm.Tool{
			&list{client: client},
			&create{client: client},
			&update{client: client},
			&remove{client: client},
		},
	}, nil
}

////////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

func (namespace) Name() string {
	return Name
}

// Description returns what the namespace's tools are for
func (namespace) Description() string {
	return "Tracks shipments, with AfterShip"
}

// SystemPrompt returns how a webhook of AfterShip's is set up, which none of
// the tools do, as the person registers it
func (namespace) SystemPrompt() string {
	return systemPrompt
}

// Type returns that the namespace's tools are built in
func (namespace) Type() string { return toolschema.NamespaceBuiltin }

// Watch sends the namespace's tools, which don't change
func (ns *namespace) Watch(ctx context.Context, ch chan<- eventschema.Change) error {
	return toolkit.WatchTools(ctx, ch, ns.Tools())
}

func (n namespace) Tools() []llm.Tool {
	return slices.Clone(n.tools)
}

////////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// unmarshal decodes the tool input, which may be empty, into v
func unmarshal(input json.RawMessage, v any) error {
	if len(input) == 0 {
		return nil
	}
	if err := json.Unmarshal(input, v); err != nil {
		return llm.ErrBadParameter.Withf("invalid input: %v", err)
	}
	return nil
}
