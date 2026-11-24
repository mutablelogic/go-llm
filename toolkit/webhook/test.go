package webhook

import (
	"context"
	"encoding/json"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	toolkit "github.com/mutablelogic/go-llm/toolkit"
	httpclient "github.com/mutablelogic/go-llm/webhook/httpclient"
	schema "github.com/mutablelogic/go-llm/webhook/schema"
	jsonschema "github.com/mutablelogic/go-server/pkg/jsonschema"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// test is the tool which makes a test call to a webhook of the session it's
// called in
type test struct {
	toolkit.Base
	client *httpclient.Client
}

var _ llm.Tool = (*test)(nil)

// testInput is the webhook, and how the call is signed, with its secret, which
// a model doesn't need
type testInput struct {
	Webhook   uint64 `json:"webhook_id" help:"Serial number of the webhook, such as from the list" min:"1"`
	Signature string `json:"signature,omitempty" enum:"good,bad,none" help:"How the call is signed: good, with the webhook's secret, which it needs to have, bad, with another secret, or none. When it isn't given, it's good, when the webhook has a secret, or else none" optional:""`
}

// testOutput is the call which was made
type testOutput struct {
	callOutput
	Note string `json:"note"`
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

func (*test) Name() string { return "test" }

func (*test) Description() string {
	return "Makes a test call to one of this conversation's webhooks, which is verified, accepted or refused, and kept, as its service's calls are, and returns it, as it was kept. It's marked as a test. With a secret, a good signature is accepted, and verified, and a bad one, or none, is refused. Without a secret, any is accepted, and not_checked, and good is an error. The webhook's URL isn't called, so it doesn't show that the service can call it, or has the same secret."
}

func (*test) InputSchema() *jsonschema.Schema {
	return jsonschema.MustFor[testInput]()
}

// Run makes a test call to the webhook, when it's of the session it's called
// in, and returns it, whether it was accepted, or refused. Another session's isn't found.
func (t *test) Run(ctx context.Context, input json.RawMessage) (any, error) {
	var req testInput
	if err := json.Unmarshal(input, &req); err != nil {
		return nil, llm.ErrBadParameter.With(err.Error())
	}
	session, err := conversation(ctx)
	if err != nil {
		return nil, err
	}
	toolkit.Logf(ctx, "Testing webhook %d", req.Webhook)
	webhook, err := sessionWebhook(ctx, t.client, session, req.Webhook)
	if err != nil {
		return nil, err
	}
	call, err := t.client.TestCall(ctx, schema.TestRequest{Webhook: webhook.ID, Signature: req.Signature})
	if err != nil {
		return nil, err
	}
	return testOutput{callOutput: newCallOutput(*call), Note: "It doesn't show that the service can call the webhook, or has the same secret."}, nil
}
