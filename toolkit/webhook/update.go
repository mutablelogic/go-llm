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
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// update is the tool which changes a webhook of the session it's called in,
// such as with the secret its service gives, once its URL is registered
type update struct {
	toolkit.Base
	client *httpclient.Client
}

var _ llm.Tool = (*update)(nil)

// updateInput is the webhook, and what's changed. A secret can't be removed,
// as a model can give an empty one, when it means none.
type updateInput struct {
	Webhook     uint64 `json:"webhook_id" help:"Serial number of the webhook, such as from the list" min:"1"`
	Description string `json:"description,omitempty" help:"What the webhook is for, or the same as before, when it isn't given" optional:""`
	Secret      string `json:"secret,omitempty" help:"The secret which the service signs its calls with, as the service gave it, which replaces any before, or the same as before, when it isn't given" optional:""`
}

// updateOutput is the webhook, as it was changed
type updateOutput struct {
	webhookOutput
	Note string `json:"note"`
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

func (*update) Name() string { return "update" }

func (*update) Description() string {
	return "Changes the description, or the secret, of one of this conversation's webhooks, and keeps its URL. Set the secret when the service gives one once the URL is registered, as Pidgeon does. From then, calls which aren't signed with it are refused. A secret can be replaced, but not removed."
}

func (*update) InputSchema() *jsonschema.Schema {
	return jsonschema.MustFor[updateInput]()
}

// Run changes the webhook, when it's of the session it's called in, and
// returns it. Another session's isn't found.
func (t *update) Run(ctx context.Context, input json.RawMessage) (any, error) {
	var req updateInput
	if err := json.Unmarshal(input, &req); err != nil {
		return nil, llm.ErrBadParameter.With(err.Error())
	}
	var change schema.WebhookRequest
	if description := strings.TrimSpace(req.Description); description != "" {
		change.Description = types.Ptr(description)
	}
	if secret := strings.TrimSpace(req.Secret); secret != "" {
		change.Secret = types.Ptr(secret)
	}
	if change.Description == nil && change.Secret == nil {
		return nil, llm.ErrBadParameter.With("a description, or a secret, is needed")
	}
	session, err := conversation(ctx)
	if err != nil {
		return nil, err
	}
	toolkit.Logf(ctx, "Changing webhook %d", req.Webhook)
	webhook, err := sessionWebhook(ctx, t.client, session, req.Webhook)
	if err != nil {
		return nil, err
	} else if webhook, err = t.client.UpdateWebhook(ctx, webhook.ID, change); err != nil {
		return nil, err
	}
	return updateOutput{webhookOutput: newWebhookOutput(*webhook), Note: secretNote(webhook.HasSecret)}, nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// secretNote returns what a model is told of a webhook which has a secret, or
// doesn't
func secretNote(secret bool) string {
	if secret {
		return "Its calls are refused unless they're signed with its secret, so its service needs the same one."
	}
	return "It's open: it has no secret, so its calls' signatures aren't checked, and anyone who has the URL can call it. Tell the person so, and if its service gives a secret once the URL is registered, set it with webhook:update."
}
