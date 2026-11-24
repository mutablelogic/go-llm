package webhook

import (
	"context"
	"encoding/json"
	"strings"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	toolkit "github.com/mutablelogic/go-llm/toolkit"
	httpclient "github.com/mutablelogic/go-llm/webhook/httpclient"
	schema "github.com/mutablelogic/go-llm/webhook/schema"
	jsonschema "github.com/mutablelogic/go-server/pkg/jsonschema"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// create is the tool which creates a webhook, for the session it's called in
type create struct {
	toolkit.Base
	client *httpclient.Client
}

var _ llm.Tool = (*create)(nil)

// createInput is the webhook, but not its session, which is the one the tool
// is called in, as a model doesn't know it
type createInput struct {
	Description string `json:"description" help:"What the webhook is for, in a few words, which says which service calls it, and when, such as GitHub: issues opened on go-llm"`
	Type        string `json:"type,omitempty" enum:"aftership,generic,github,pidgeon" help:"The service which calls it, which says how its calls are verified, and what they are: aftership, github or pidgeon, or generic, for any other, which it is without one" optional:""`
	Secret      string `json:"secret,omitempty" help:"The secret which the service signs its calls with, which is optional. The service needs the same one, so give it when you register the URL, or else use the one which the service gave. Without one, the webhook works, but its calls' signatures aren't checked." optional:""`
}

// createOutput is the webhook which was created, as a model is told it
type createOutput struct {
	webhookOutput
	URL   string `json:"url,omitempty"`
	Token string `json:"token,omitempty"`
	Note  string `json:"note"`
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

func (*create) Name() string { return "create" }

func (*create) Description() string {
	return "Creates a webhook for this conversation, which is a URL that an external service calls when something happens, such as GitHub, when an issue is opened, and returns its URL, for you to register with the service, with the service's own tools. Only create one when you have a tool which registers it. Don't give the URL to the person, as whoever has it can call the webhook. What it's sent is kept, but doesn't come to this conversation yet, so don't say that you'll be told of it."
}

func (*create) InputSchema() *jsonschema.Schema {
	return jsonschema.MustFor[createInput]()
}

// Run creates the webhook, for the session it's called in, and returns it,
// with its URL, or its token, when the webhook server has no public endpoint
func (t *create) Run(ctx context.Context, input json.RawMessage) (any, error) {
	var req createInput
	if err := json.Unmarshal(input, &req); err != nil {
		return nil, llm.ErrBadParameter.With(err.Error())
	}
	session, err := conversation(ctx)
	if err != nil {
		return nil, err
	}
	toolkit.Logf(ctx, "Creating a webhook for %s", strings.TrimSpace(req.Description))
	webhook, err := t.client.CreateWebhook(ctx, schema.WebhookMeta{Session: session, Description: req.Description, Type: req.Type, Secret: req.Secret})
	if err != nil {
		return nil, err
	}

	// The token is only returned now, in its URL, when it has one
	result := createOutput{webhookOutput: newWebhookOutput(webhook.Webhook), URL: webhook.URL}
	notes := []string{"Register the URL with the service now, as it is, as it can't be seen again. Don't give it to the person, or call it, or fetch it: webhook:test makes a test call."}
	if webhook.URL == "" {
		result.Token = webhook.Token
		notes = []string{"The webhook server has no public address, so a service on the internet can't call it. Its URL is the server's, then /hook/, and the token, which can't be seen again. Don't give it to the person."}
	}
	if webhook.HasSecret {
		notes = append(notes, "Its calls are refused unless they're signed with the secret, so register the same one with the service.")
	} else {
		notes = append(notes, "It's open: it has no secret, so its calls' signatures aren't checked, and anyone who has the URL can call it. Tell the person so.")
	}
	notes = append(notes, "Its calls are kept, but don't come to this conversation yet.")
	result.Note = strings.Join(notes, " ")
	return result, nil
}
