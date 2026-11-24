package webhook

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"
	"uuid"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/webhook/schema"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

func Test_List(t *testing.T) {
	s := newServer(t)
	tk, session := newToolkit(t, s), uuid.New()
	call := func(ctx context.Context, input string) (result listOutput, text string, err error) {
		data, err := tk.CallTool(ctx, "webhook:list", json.RawMessage(input))
		if err == nil {
			require.NoError(t, json.Unmarshal(data, &result))
		}
		return result, string(data), err
	}
	ids := func(calls []callOutput) (result []uint64) {
		for _, c := range calls {
			result = append(result, c.ID)
		}
		return
	}

	// None, which are empty lists
	ctx := inSession(t.Context(), session)
	_, text, err := call(ctx, `{}`)
	require.NoError(t, err)
	assert.JSONEq(t, `{"webhooks_count":0,"webhooks":[],"calls_count":0,"calls":[]}`, text)

	// This session's webhooks, newest first, and their calls, newest first,
	// but not another session's
	create := func(session uuid.UUID, description, secret string) schema.WebhookToken {
		w, err := s.registry.CreateWebhook(t.Context(), schema.WebhookMeta{Session: session, Description: description, Type: "github", Secret: secret})
		require.NoError(t, err)
		return w
	}
	a, signed, other := create(session, "Issues", ""), create(session, "Signed", "s3cret"), create(uuid.New(), "Another's", "")
	req := newRequest(`{"action":"opened","text":"what was sent"}`)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-GitHub-Event", "issues")
	_, err = s.registry.Call(t.Context(), a.Token, req)
	require.NoError(t, err)
	s.call(t, other, "another's")
	s.call(t, signed, "not signed")
	result, text, err := call(ctx, `{}`)
	require.NoError(t, err)
	assert.Equal(t, uint(2), result.WebhooksCount)
	require.Len(t, result.Webhooks, 2)
	assert.Equal(t, newWebhookOutput(signed.Webhook), result.Webhooks[0])
	assert.Equal(t, newWebhookOutput(a.Webhook), result.Webhooks[1])
	assert.Equal(t, uint(2), result.CallsCount)
	require.Equal(t, []uint64{3, 1}, ids(result.Calls))
	assert.Equal(t, signed.ID, result.Calls[0].Webhook)
	assert.Equal(t, "refused", result.Calls[0].Status)
	assert.Contains(t, result.Calls[0].Error, "X-Hub-Signature-256")
	assert.Equal(t, "issues.opened", result.Calls[1].Event)
	assert.Equal(t, "accepted", result.Calls[1].Status)
	assert.Equal(t, "failed", result.Calls[0].Signature)
	assert.Equal(t, "not_checked", result.Calls[1].Signature)
	assert.True(t, result.Webhooks[0].HasSecret)
	assert.False(t, result.Webhooks[1].HasSecret)
	assert.NotEmpty(t, result.Calls[1].Received)
	for _, hidden := range []string{"what was sent", "not signed", "nother", "s3cret", session.String(), a.Token} {
		assert.NotContains(t, text, hidden)
	}

	// One webhook's, and those which were accepted, or refused
	result, _, err = call(ctx, fmt.Sprintf(`{"webhook_id":%d}`, a.ID))
	require.NoError(t, err)
	assert.Equal(t, uint(1), result.WebhooksCount)
	assert.Equal(t, []webhookOutput{newWebhookOutput(a.Webhook)}, result.Webhooks)
	assert.Equal(t, []uint64{1}, ids(result.Calls))
	result, _, err = call(ctx, `{"call_status":"refused"}`)
	require.NoError(t, err)
	assert.Equal(t, []uint64{3}, ids(result.Calls))

	// Those received since
	for since, expected := range map[string][]uint64{"1h": {3, 1}, " 1d12h ": {3, 1}, "1w": {3, 1}, "": {3, 1}} {
		result, _, err = call(ctx, fmt.Sprintf(`{"call_since":%q}`, since))
		require.NoError(t, err, since)
		assert.Equal(t, expected, ids(result.Calls), since)
	}
	_, err = s.store.CreateCall(t.Context(), schema.Call{Webhook: a.ID, Method: "POST", Received: time.Now().Add(-2 * time.Hour)})
	require.NoError(t, err)
	result, _, err = call(ctx, `{"call_since":"1h"}`)
	require.NoError(t, err)
	assert.Equal(t, uint(2), result.WebhooksCount)
	assert.Equal(t, uint(2), result.CallsCount)
	assert.Equal(t, []uint64{3, 1}, ids(result.Calls))
	result, _, err = call(ctx, `{"call_since":"3h"}`)
	require.NoError(t, err)
	assert.Equal(t, []uint64{4, 3, 1}, ids(result.Calls))
	for _, since := range []string{"60", "yesterday", "-1h", "0s", "1x"} {
		_, _, err = call(ctx, fmt.Sprintf(`{"call_since":%q}`, since))
		assert.ErrorIs(t, err, llm.ErrBadParameter, since)
	}

	// Not another session's, which is as one which there isn't
	_, _, err = call(ctx, fmt.Sprintf(`{"webhook_id":%d}`, other.ID))
	assert.ErrorIs(t, err, llm.ErrNotFound)
	_, _, err = call(ctx, `{"webhook_id":999}`)
	assert.ErrorIs(t, err, llm.ErrNotFound)

	// The most recent, of more, by default, and up to the limit
	for range 30 {
		s.call(t, a, "more")
	}
	result, _, err = call(ctx, `{}`)
	require.NoError(t, err)
	assert.Equal(t, uint(33), result.CallsCount)
	require.Len(t, result.Calls, callsDefault)
	assert.Equal(t, uint64(34), result.Calls[0].ID)
	result, _, err = call(ctx, `{"call_limit":2}`)
	require.NoError(t, err)
	assert.Equal(t, []uint64{34, 33}, ids(result.Calls))
	_, _, err = call(ctx, `{"call_limit":21}`)
	assert.Error(t, err)

	// One which was refused, though its signature wasn't checked, as it isn't
	// GitHub's, and though it was verified
	s.call(t, a, "not GitHub's")
	result, _, err = call(ctx, `{"call_status":"refused","call_limit":1}`)
	require.NoError(t, err)
	assert.Equal(t, "not_checked", result.Calls[0].Signature)
	assert.Contains(t, result.Calls[0].Error, "X-GitHub-Event")
	_, err = s.store.CreateCall(t.Context(), schema.Call{Webhook: signed.ID, Method: "POST", Verified: true, Error: llm.ErrBadParameter.With("not JSON").Error()})
	require.NoError(t, err)
	result, _, err = call(ctx, `{"call_status":"refused","call_limit":1}`)
	require.NoError(t, err)
	assert.Equal(t, "verified", result.Calls[0].Signature)

	// One whose signature was verified
	_, err = s.store.CreateCall(t.Context(), schema.Call{Webhook: signed.ID, Method: "POST", Verified: true})
	require.NoError(t, err)
	result, _, err = call(ctx, `{"call_limit":1}`)
	require.NoError(t, err)
	assert.Equal(t, "verified", result.Calls[0].Signature)

	// Every webhook, of more than the server lists at once
	for range 110 {
		newWebhook(t, s, session)
	}
	result, _, err = call(ctx, `{}`)
	require.NoError(t, err)
	assert.Equal(t, uint(112), result.WebhooksCount)
	require.Len(t, result.Webhooks, 112)
	seen := map[uint64]bool{}
	for _, webhook := range result.Webhooks {
		seen[webhook.ID] = true
	}
	assert.Len(t, seen, 112)
	assert.Equal(t, a.ID, result.Webhooks[111].ID)

	// Not without a session, or when the server can't be reached
	_, _, err = call(t.Context(), `{}`)
	assert.ErrorIs(t, err, llm.ErrBadParameter)
	_, _, err = call(ctx, `{"call_status":"pending"}`)
	assert.Error(t, err)
	s.down.Store(true)
	_, _, err = call(ctx, `{}`)
	assert.Error(t, err)
}
