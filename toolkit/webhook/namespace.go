// Package webhook is the namespace of the webhooks' tools, of a webhook
// server, whose URLs external services call, and which is watched for their
// calls
package webhook

import (
	"context"
	"errors"
	"slices"
	"strings"
	"sync"
	"time"
	"uuid"

	// Packages
	client "github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
	toolkit "github.com/mutablelogic/go-llm/toolkit"
	toolschema "github.com/mutablelogic/go-llm/toolkit/schema"
	httpclient "github.com/mutablelogic/go-llm/webhook/httpclient"
	schema "github.com/mutablelogic/go-llm/webhook/schema"
	eventschema "github.com/mutablelogic/go-server/broadcaster/schema"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Namespace is the namespace of the webhooks' tools, of a webhook server
type Namespace struct {
	client *httpclient.Client
	tools  []llm.Tool

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

// webhookOutput is a webhook, as a model is told it, without its session, and
// whether it has a secret, which its calls' signatures are checked with
type webhookOutput struct {
	ID          uint64 `json:"id"`
	Type        string `json:"type"`
	Description string `json:"description"`
	HasSecret   bool   `json:"has_secret"`
	Created     string `json:"created"`
}

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

// Name is the name of the namespace of the webhooks' tools
const Name = "webhook"

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// New returns the namespace of the webhook server's API, at the endpoint,
// such as http://localhost:8087/api, with the client options. The server
// isn't called until the namespace is run, or a tool is called.
func New(endpoint string, opts ...client.ClientOpt) (*Namespace, error) {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return nil, llm.ErrBadParameter.With("missing webhook endpoint")
	}

	c, err := httpclient.New(endpoint, opts...)
	if err != nil {
		return nil, err
	}
	return &Namespace{client: c, tools: []llm.Tool{&create{client: c}, &update{client: c}, &list{client: c}, &get{client: c}, &remove{client: c}, &test{client: c}}}, nil
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

// SystemPrompt returns no system prompt, as the tools' descriptions say how
// to use them
func (*Namespace) SystemPrompt() string { return "" }

// Tools returns the namespace's tools
func (ns *Namespace) Tools() []llm.Tool { return slices.Clone(ns.tools) }

// Watch sends the namespace's tools, which don't change
func (ns *Namespace) Watch(ctx context.Context, ch chan<- eventschema.Change) error {
	return toolkit.WatchTools(ctx, ch, ns.Tools())
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// conversation returns the session which a tool is called in, whose webhooks
// it's for, or ErrBadParameter when it isn't called in one
func conversation(ctx context.Context) (uuid.UUID, error) {
	session := toolkit.SessionFromContext(ctx).ID()
	if session == uuid.Nil() {
		return uuid.Nil(), llm.ErrBadParameter.With("a webhook is a conversation's, and this isn't one")
	}
	return session, nil
}

// sessionWebhook returns the webhook with the serial number, or ErrNotFound
// when there's none, or it's another session's
func sessionWebhook(ctx context.Context, c *httpclient.Client, session uuid.UUID, id uint64) (*schema.Webhook, error) {
	webhook, err := c.GetWebhook(ctx, id)
	if isNotFound(err) || (err == nil && webhook.Session != session) {
		return nil, llm.ErrNotFound.Withf("webhook %d", id)
	}
	return webhook, err
}

// sessionCall returns the call with the serial number, or ErrNotFound when
// there's none, or it's of another session's webhook
func sessionCall(ctx context.Context, c *httpclient.Client, session uuid.UUID, id uint64) (*schema.Call, error) {
	call, err := c.GetCall(ctx, id)
	if err == nil {
		_, err = sessionWebhook(ctx, c, session, call.Webhook)
	}
	if isNotFound(err) || errors.Is(err, llm.ErrNotFound) {
		return nil, llm.ErrNotFound.Withf("call %d", id)
	}
	return call, err
}

// newWebhookOutput returns the webhook, as a model is told it
func newWebhookOutput(webhook schema.Webhook) webhookOutput {
	return webhookOutput{ID: webhook.ID, Type: webhook.Type, Description: webhook.Description, HasSecret: webhook.HasSecret, Created: webhook.Created.UTC().Format(time.RFC3339)}
}
