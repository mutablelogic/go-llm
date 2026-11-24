package webhook

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
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

	// Its token is hidden in its query, and however it's escaped, but what
	// was sent is kept as it was
	query := url.Values{"u": {"https://example.com/api/hook/" + a.Token}, "hook/" + a.Token: {"1"}, "e": {"https%3A%2F%2Fexample.com%2Fhook%2F" + a.Token}}
	body := `{"unicode":"https:\u002f\u002fexample.com\u002fhook\u002f` + a.Token + `",
		"upper":"hook\u002F` + a.Token + `",
		"url":["https%3a%2f%2fexample.com%2fapi%2fhook%2f` + a.Token + `"],
		"twice":{"https%253A%252F%252Fexample.com%252Fhook%252F` + a.Token + `":1},
		"json":"{\"url\":\"https:\\/\\/example.com\\/hook\\/` + a.Token + `\"}",
		"json2":"{\"url\":\"hook\\u002f` + a.Token + `\"}",
		"event":"hook/` + a.Token + `",
		"large":12345678901234567890}`
	req = newRequest(body)
	req.URL.RawQuery = query.Encode()
	req.Header.Set("Content-Type", "application/json")
	received, err := s.registry.Call(t.Context(), a.Token, req)
	require.NoError(t, err)
	result, text, err = get(ctx, fmt.Sprintf(`{"call_id":%d}`, received.Call.ID))
	require.NoError(t, err)
	assert.NotContains(t, text, a.Token)
	assert.Equal(t, 10, strings.Count(text, "{token}"), text)
	assert.Contains(t, text, "12345678901234567890")
	kept, err := s.registry.GetCall(t.Context(), received.Call.ID)
	require.NoError(t, err)
	assert.Contains(t, string(kept.Body), a.Token)
	assert.Contains(t, kept.Query.Get("u"), a.Token)

	// Its event, and delivery, which a service says
	event, err := s.store.CreateCall(t.Context(), schema.Call{Webhook: a.ID, Method: "POST", Event: "hook/" + a.Token, Delivery: "hook/" + a.Token, Error: "hook/" + a.Token})
	require.NoError(t, err)
	_, text, err = get(ctx, fmt.Sprintf(`{"call_id":%d}`, event.ID))
	require.NoError(t, err)
	assert.NotContains(t, text, a.Token)

	// Which isn't text, so how long it was
	binary, err := s.store.CreateCall(t.Context(), schema.Call{Webhook: a.ID, Method: "POST", Body: []byte{0xff, 0xfe, 0x00}})
	require.NoError(t, err)
	result, _, err = get(ctx, fmt.Sprintf(`{"call_id":%d}`, binary.ID))
	require.NoError(t, err)
	assert.Empty(t, result.Values)
	assert.Equal(t, 3, result.Bytes)

	// Another session's is as one which there isn't
	s.call(t, other, "another's")
	_, _, err = get(ctx, `{"call_id":5}`)
	assert.ErrorIs(t, err, llm.ErrNotFound)
	assert.ErrorContains(t, err, "call 5")
	_, _, err = get(ctx, `{"call_id":6}`)
	assert.ErrorIs(t, err, llm.ErrNotFound)
	assert.ErrorContains(t, err, "call 6")

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
