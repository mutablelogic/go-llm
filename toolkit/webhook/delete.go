package webhook

import (
	"context"
	"encoding/json"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	toolkit "github.com/mutablelogic/go-llm/toolkit"
	httpclient "github.com/mutablelogic/go-llm/webhook/httpclient"
	jsonschema "github.com/mutablelogic/go-server/pkg/jsonschema"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// remove is the tool which deletes a webhook of the session it's called in,
// with its calls, or a call of one
type remove struct {
	toolkit.Base
	client *httpclient.Client
}

var _ llm.Tool = (*remove)(nil)

// removeInput is the webhook, or the call
type removeInput struct {
	Webhook uint64 `json:"webhook_id,omitempty" help:"Serial number of the webhook to delete, with its calls, or else give the call_id" optional:""`
	Call    uint64 `json:"call_id,omitempty" help:"Serial number of the call to delete, or else give the webhook_id" optional:""`
}

// removeOutput is the webhook, or the call, as it was
type removeOutput struct {
	Webhook *webhookOutput `json:"webhook,omitempty"`
	Call    *callOutput    `json:"call,omitempty"`
	Note    string         `json:"note"`
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

func (*remove) Name() string { return "delete" }

func (*remove) Description() string {
	return "Deletes one of this conversation's webhooks, with every call of it, so its URL stops working at once, or one call of a webhook. It can't be undone. Only delete what the person asks you to, or a webhook which you created, and couldn't register with its service."
}

func (*remove) InputSchema() *jsonschema.Schema {
	return jsonschema.MustFor[removeInput]()
}

// Run deletes the webhook, with its calls, or the call, when it's of the
// session it's called in, and returns it, as it was. Another session's isn't
// found.
func (t *remove) Run(ctx context.Context, input json.RawMessage) (any, error) {
	var req removeInput
	if err := json.Unmarshal(input, &req); err != nil {
		return nil, llm.ErrBadParameter.With(err.Error())
	} else if (req.Webhook == 0) == (req.Call == 0) {
		return nil, llm.ErrBadParameter.With("either the webhook_id, or the call_id, is needed")
	}
	session, err := conversation(ctx)
	if err != nil {
		return nil, err
	}

	// The call
	if req.Call > 0 {
		toolkit.Logf(ctx, "Deleting webhook call %d", req.Call)
		call, err := sessionCall(ctx, t.client, session, req.Call)
		if err != nil {
			return nil, err
		} else if call, err = t.client.DeleteCall(ctx, call.ID); err != nil {
			return nil, err
		}
		output := newCallOutput(*call)
		return removeOutput{Call: &output, Note: "The call was deleted."}, nil
	}

	// The webhook, with its calls
	toolkit.Logf(ctx, "Deleting webhook %d", req.Webhook)
	webhook, err := sessionWebhook(ctx, t.client, session, req.Webhook)
	if err != nil {
		return nil, err
	} else if webhook, err = t.client.DeleteWebhook(ctx, webhook.ID); err != nil {
		return nil, err
	}
	output := newWebhookOutput(*webhook)
	return removeOutput{Webhook: &output, Note: "The webhook, and its calls, were deleted, and its URL no longer works. If it's registered with its service, remove it there too."}, nil
}
