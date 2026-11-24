package webhook

import (
	"context"
	"encoding/json"
	"strings"
	"time"

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

// list is the tool which lists the webhooks of the session it's called in,
// and the calls of them which were received most recently
type list struct {
	toolkit.Base
	client *httpclient.Client
}

var _ llm.Tool = (*list)(nil)

// listInput is which webhook, of the session's, and which of its calls, are
// listed. Only which webhook it is changes the webhooks which are listed, so
// the rest say they're of the calls.
type listInput struct {
	Webhook uint64 `json:"webhook_id,omitempty" help:"Serial number of the only webhook to list, with its calls, rather than each of this conversation's" optional:""`
	Since   string `json:"call_since,omitempty" help:"How long ago the calls to list were received, at most, such as 30m, 1h, 1d12h or 1w, or any time, when it isn't given" example:"1h" optional:""`
	Status  string `json:"call_status,omitempty" enum:"accepted,refused" help:"Which calls to list: accepted, or refused, such as those which weren't signed, or both, when it isn't given" optional:""`
	Limit   uint64 `json:"call_limit,omitempty" help:"Most calls to list, which is 10 when it isn't given" min:"1" max:"20" optional:""`
}

// callOutput is a call, as a model is told it, without what was sent. Its
// signature is only there when it was accepted, as why it was refused says
// what was wrong with it.
type callOutput struct {
	ID        uint64 `json:"id"`
	Webhook   uint64 `json:"webhook_id"`
	Received  string `json:"received"`
	Event     string `json:"event,omitempty"`
	Status    string `json:"status"`
	Test      bool   `json:"test,omitempty"`
	Signature string `json:"signature,omitempty"`
	Error     string `json:"error,omitempty"`
}

// listOutput is the webhooks, and the calls of them, newest first, and how
// many there are of each, by what they count, as with filters of calls, there
// can be webhooks, and no calls
type listOutput struct {
	WebhooksCount uint            `json:"webhooks_count"`
	Webhooks      []webhookOutput `json:"webhooks"`
	CallsCount    uint            `json:"calls_count"`
	Calls         []callOutput    `json:"calls"`
}

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// callsDefault is the calls which are listed, without a limit
	callsDefault = 10

	// callsLimit is the most calls which are listed
	callsLimit = 20

	// The signature of a call which was accepted, which was verified, or
	// which wasn't checked, as its webhook had no secret when it was received
	signatureVerified   = "verified"
	signatureNotChecked = "not_checked"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

func (*list) Name() string { return "list" }

func (*list) Description() string {
	return "Lists this conversation's webhooks, and the most recent calls of them, newest first, such as those of the past hour: when each was received, its event, which is what its service says happened, whether it was accepted, or refused, and why, and the signature of one which was accepted: verified, or not_checked, when the webhook had no secret to check it with, so anyone who has its URL could have sent it. A test is a call which webhook:test made, and no service sent. The filters of calls don't change which webhooks are listed. A call's id is a serial number across every webhook, so isn't how many there are, which is calls_count. What a call sent, which webhook:get returns, and a webhook's URL, aren't listed."
}

func (*list) InputSchema() *jsonschema.Schema {
	return jsonschema.MustFor[listInput]()
}

// Run returns the webhooks of the session it's called in, and their most
// recent calls, or those of one of its webhooks. Another session's webhook
// isn't found.
func (t *list) Run(ctx context.Context, input json.RawMessage) (any, error) {
	var req listInput
	if err := json.Unmarshal(input, &req); err != nil {
		return nil, llm.ErrBadParameter.With(err.Error())
	}
	since, err := sinceTime(req.Since)
	if err != nil {
		return nil, err
	}
	session, err := conversation(ctx)
	if err != nil {
		return nil, err
	}
	result := listOutput{Webhooks: []webhookOutput{}, Calls: []callOutput{}}

	// The webhooks, or the one, when it's the session's
	if req.Webhook > 0 {
		toolkit.Logf(ctx, "Looking at webhook %d", req.Webhook)
		webhook, err := sessionWebhook(ctx, t.client, session, req.Webhook)
		if err != nil {
			return nil, err
		}
		result.WebhooksCount, result.Webhooks = 1, append(result.Webhooks, newWebhookOutput(*webhook))
	} else {
		toolkit.Logf(ctx, "Looking at the webhooks")
		for {
			webhooks, err := t.client.ListWebhooks(ctx, schema.WebhookListRequest{Session: session.String(), Offset: uint64(len(result.Webhooks))})
			if err != nil {
				return nil, err
			}
			result.WebhooksCount = webhooks.Count
			for _, webhook := range webhooks.Body {
				result.Webhooks = append(result.Webhooks, newWebhookOutput(webhook))
			}
			if len(webhooks.Body) == 0 || uint(len(result.Webhooks)) >= webhooks.Count {
				break
			}
		}
	}

	// Their most recent calls
	limit := req.Limit
	if limit == 0 {
		limit = callsDefault
	}
	calls, err := t.client.ListCalls(ctx, schema.CallListRequest{
		Session: session.String(),
		Webhook: req.Webhook,
		Since:   since,
		Status:  req.Status,
		Newest:  true,
		Limit:   types.Ptr(min(limit, callsLimit)),
	})
	if err != nil {
		return nil, err
	}
	result.CallsCount = calls.Count
	for _, call := range calls.Body {
		result.Calls = append(result.Calls, newCallOutput(call))
	}
	return result, nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// newCallOutput returns the call, as a model is told it, without what was
// sent
func newCallOutput(call schema.Call) callOutput {
	return callOutput{
		ID:        call.ID,
		Webhook:   call.Webhook,
		Received:  call.Received.UTC().Format(time.RFC3339),
		Event:     hideToken(call.Event),
		Status:    call.Status(),
		Test:      call.Test,
		Signature: signature(call),
		Error:     hideToken(call.Error),
	}
}

// signature returns the signature of a call which was accepted, or empty. One
// which isn't verified is only accepted when its webhook has no secret.
func signature(call schema.Call) string {
	switch {
	case call.Status() != schema.CallAccepted:
		return ""
	case call.Verified:
		return signatureVerified
	default:
		return signatureNotChecked
	}
}

// sinceTime returns the time which is the duration ago, such as 1h or 7d, or
// zero, when it's empty, or ErrBadParameter, when it isn't a duration, or is
// less than a second, such as a number without a unit
func sinceTime(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, nil
	}
	duration, err := types.ParseDuration(value)
	if err != nil || duration < time.Second {
		return time.Time{}, llm.ErrBadParameter.Withf("call_since is %q, which isn't a duration, such as 30m, 1h or 7d", value)
	}
	return time.Now().Add(-duration), nil
}
