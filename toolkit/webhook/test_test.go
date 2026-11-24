package webhook

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"uuid"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/webhook/schema"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

func Test_Test(t *testing.T) {
	s := newServer(t)
	tk, session := newToolkit(t, s), uuid.New()
	ctx := inSession(t.Context(), session)
	call := func(ctx context.Context, name, input string, result any) error {
		data, err := tk.CallTool(ctx, "webhook:"+name, json.RawMessage(input))
		if err == nil {
			require.NoError(t, json.Unmarshal(data, result))
		}
		return err
	}
	a, other := newWebhook(t, s, session), newWebhook(t, s, uuid.New())

	// A test call, which is listed, and got, as one
	var tested testOutput
	require.NoError(t, call(ctx, "test", fmt.Sprintf(`{"webhook_id":%d}`, a.ID), &tested))
	assert.Equal(t, a.ID, tested.Webhook)
	assert.True(t, tested.Test)
	assert.Equal(t, "accepted", tested.Status)
	assert.Equal(t, "not_checked", tested.Signature)
	s.call(t, a, "real")
	var listed listOutput
	require.NoError(t, call(ctx, "list", `{}`, &listed))
	require.Len(t, listed.Calls, 2)
	assert.False(t, listed.Calls[0].Test)
	assert.Equal(t, "not_checked", listed.Calls[0].Signature)
	assert.Equal(t, tested.callOutput, listed.Calls[1])
	var got getOutput
	require.NoError(t, call(ctx, "get", fmt.Sprintf(`{"call_id":%d}`, tested.ID), &got))
	assert.True(t, got.Test)
	assert.Contains(t, string(got.Values), "test call")

	// With a secret, each signature can be told from the others
	signed, err := s.registry.CreateWebhook(t.Context(), schema.WebhookMeta{Session: session, Description: "Signed", Type: "github", Secret: "s3cret"})
	require.NoError(t, err)
	outcomes := map[string]string{}
	for _, signature := range []string{"good", "bad", "none"} {
		var result testOutput
		require.NoError(t, call(ctx, "test", fmt.Sprintf(`{"webhook_id":%d,"signature":%q}`, signed.ID, signature), &result), signature)
		assert.True(t, result.Test)
		outcomes[signature] = result.Status + " " + result.Signature + result.Error
	}
	assert.Equal(t, "accepted verified", outcomes["good"])
	assert.Contains(t, outcomes["bad"], "refused")
	assert.Contains(t, outcomes["bad"], "isn't the body's")
	assert.Contains(t, outcomes["none"], "refused")
	assert.Contains(t, outcomes["none"], "has no X-Hub-Signature-256")
	assert.ErrorContains(t, call(ctx, "test", fmt.Sprintf(`{"webhook_id":%d,"signature":"good"}`, a.ID), &tested), "no secret")
	assert.Error(t, call(ctx, "test", fmt.Sprintf(`{"webhook_id":%d,"signature":"other"}`, a.ID), &tested))

	// Not another session's, or without a webhook, or a session
	for _, id := range []uint64{other.ID, 999} {
		assert.ErrorIs(t, call(ctx, "test", fmt.Sprintf(`{"webhook_id":%d}`, id), &tested), llm.ErrNotFound)
	}
	assert.Error(t, call(ctx, "test", `{}`, &tested))
	assert.ErrorIs(t, call(t.Context(), "test", fmt.Sprintf(`{"webhook_id":%d}`, a.ID), &tested), llm.ErrBadParameter)
	list, err := s.registry.ListCalls(t.Context(), schema.CallListRequest{})
	require.NoError(t, err)
	assert.Equal(t, uint(5), list.Count)
}
