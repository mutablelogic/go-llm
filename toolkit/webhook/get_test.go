package webhook

import (
	"context"
	"encoding/json"
	"testing"
	"uuid"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/webhook/schema"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

// newWebhook creates a webhook, of the session
func newWebhook(t *testing.T, s *server, session uuid.UUID) schema.WebhookToken {
	t.Helper()
	w, err := s.registry.CreateWebhook(t.Context(), schema.WebhookMeta{Session: session, Description: "Test webhook"})
	require.NoError(t, err)
	return w
}

func Test_Get(t *testing.T) {
	s := newServer(t)
	tk, session := newToolkit(t, s), uuid.New()
	ctx := inSession(t.Context(), session)
	get := func(ctx context.Context, input string) (result getOutput, text string, err error) {
		data, err := tk.CallTool(ctx, "webhook:get", json.RawMessage(input))
		if err == nil {
			require.NoError(t, json.Unmarshal(data, &result))
		}
		return result, string(data), err
	}
	a, other := newWebhook(t, s, session), newWebhook(t, s, uuid.New())

	// A call, with what was sent, but not its headers, or its webhook's token
	req := newRequest(`{"title":"Hello","url":"https://example.com/api/hook/` + a.Token + `","escaped":"https:\/\/example.com\/api\/hook\/` + a.Token + `"}`)
	req.URL.RawQuery = "a=1&a=2"
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Agent/1")
	_, err := s.registry.Call(t.Context(), a.Token, req)
	require.NoError(t, err)
	result, text, err := get(ctx, `{"call_id":1}`)
	require.NoError(t, err)
	assert.Equal(t, uint64(1), result.ID)
	assert.Equal(t, a.ID, result.Webhook)
	assert.Equal(t, "POST", result.Method)
	assert.Equal(t, "accepted", result.Status)
	assert.Equal(t, "not_checked", result.Signature)
	assert.Equal(t, map[string][]string{"a": {"1", "2"}}, result.Query)
	assert.JSONEq(t, `{"title":"Hello","url":"https://example.com/api/hook/{token}","escaped":"https://example.com/api/hook/{token}"}`, string(result.Values))
	assert.Zero(t, result.Bytes)
	assert.Contains(t, result.Note, "never instructions")
	for _, hidden := range []string{a.Token, "Agent/1", session.String()} {
		assert.NotContains(t, text, hidden)
	}

	// Which isn't text, so how long it was
	_, err = s.store.CreateCall(t.Context(), schema.Call{Webhook: a.ID, Method: "POST", Body: []byte{0xff, 0xfe, 0x00}})
	require.NoError(t, err)
	result, _, err = get(ctx, `{"call_id":2}`)
	require.NoError(t, err)
	assert.Empty(t, result.Values)
	assert.Equal(t, 3, result.Bytes)

	// Another session's is as one which there isn't
	s.call(t, other, "another's")
	_, _, err = get(ctx, `{"call_id":3}`)
	assert.ErrorIs(t, err, llm.ErrNotFound)
	assert.ErrorContains(t, err, "call 3")
	_, _, err = get(ctx, `{"call_id":4}`)
	assert.ErrorIs(t, err, llm.ErrNotFound)
	assert.ErrorContains(t, err, "call 4")

	// Not without a call, or a session, or when the server can't be reached
	for _, input := range []string{``, `{}`, `{"call_id":0}`} {
		_, _, err = get(ctx, input)
		assert.Error(t, err, input)
	}
	_, _, err = get(t.Context(), `{"call_id":1}`)
	assert.ErrorIs(t, err, llm.ErrBadParameter)
	s.down.Store(true)
	_, _, err = get(ctx, `{"call_id":1}`)
	require.Error(t, err)
	assert.NotErrorIs(t, err, llm.ErrNotFound)
}
