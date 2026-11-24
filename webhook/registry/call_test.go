package registry_test

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"uuid"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	registry "github.com/mutablelogic/go-llm/webhook/registry"
	schema "github.com/mutablelogic/go-llm/webhook/schema"
	store "github.com/mutablelogic/go-llm/webhook/store/sqlite"
	sqlite "github.com/mutablelogic/go-sqlite"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

// githubRequest returns a request, as GitHub sends it, signed with the
// secret, unless it's empty
func githubRequest(body, secret, delivery string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/webhook/token?x=1", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "GitHub-Hookshot/1")
	req.Header.Set("X-GitHub-Event", "issues")
	req.Header.Set("X-GitHub-Delivery", delivery)
	req.Header.Set("Cookie", "not kept")
	if secret != "" {
		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write([]byte(body))
		req.Header.Set("X-Hub-Signature-256", "sha256="+hex.EncodeToString(mac.Sum(nil)))
	}
	return req
}

func Test_Registry_Call(t *testing.T) {
	ctx := context.Background()
	r, s := newRegistry(t)
	hook, err := r.CreateWebhook(ctx, schema.WebhookMeta{Session: uuid.New(), Description: "GitHub", Type: "github", Secret: "s3cret"})
	require.NoError(t, err)
	body := `{"action":"opened","issue":{"title":"Hello"}}`

	// A call, which is verified, and kept, as it was received, with what
	// happened
	received, err := r.Call(ctx, hook.Token, githubRequest(body, "s3cret", "d1"))
	require.NoError(t, err)
	assert.False(t, received.IsHandshake)
	assert.False(t, received.Duplicate)
	call := received.Call
	assert.NotZero(t, call.ID)
	assert.Equal(t, hook.ID, call.Webhook)
	assert.Equal(t, http.MethodPost, call.Method)
	assert.Equal(t, "1", call.Query.Get("x"))
	assert.Equal(t, "issues.opened", call.Event)
	assert.Equal(t, "d1", call.Delivery)
	assert.True(t, call.Verified)
	assert.JSONEq(t, body, string(call.Values))
	assert.Equal(t, body, string(call.Body))
	assert.Equal(t, "issues", call.Headers["X-Github-Event"])
	assert.NotContains(t, call.Headers, "Cookie")
	stored, err := s.GetCall(ctx, call.ID)
	require.NoError(t, err)
	assert.Equal(t, call, stored)

	// The same delivery, sent again, isn't kept again
	received, err = r.Call(ctx, hook.Token, githubRequest(body, "s3cret", "d1"))
	require.NoError(t, err)
	assert.True(t, received.Duplicate)
	assert.Zero(t, received.Call.ID)

	// A call which isn't signed with the secret isn't kept
	_, err = r.Call(ctx, hook.Token, githubRequest(body, "wrong", "d2"))
	assert.ErrorIs(t, err, llm.ErrForbidden)
	_, err = r.Call(ctx, hook.Token, githubRequest(body, "", "d3"))
	assert.ErrorIs(t, err, llm.ErrForbidden)
	list, err := r.ListCalls(ctx, schema.CallListRequest{Webhook: hook.ID, Status: schema.CallAccepted})
	require.NoError(t, err)
	assert.Equal(t, uint(1), list.Count)

	// The calls which were refused are kept, with why, and what they were,
	// and listed by their status, or with those which were accepted
	list, err = r.ListCalls(ctx, schema.CallListRequest{Webhook: hook.ID, Status: schema.CallRefused})
	require.NoError(t, err)
	require.Equal(t, uint(2), list.Count)
	for _, refused := range list.Body {
		assert.False(t, refused.Verified)
		assert.Contains(t, refused.Error, "X-Hub-Signature-256")
		assert.Equal(t, "issues.opened", refused.Event)
		assert.Equal(t, body, string(refused.Body))
	}
	assert.Equal(t, []string{"d2", "d3"}, []string{list.Body[0].Delivery, list.Body[1].Delivery})
	list, err = r.ListCalls(ctx, schema.CallListRequest{Webhook: hook.ID, After: call.ID})
	require.NoError(t, err)
	assert.Equal(t, uint(2), list.Count)

	// A delivery which was refused is accepted, when it's sent again, and
	// is right
	received, err = r.Call(ctx, hook.Token, githubRequest(body, "s3cret", "d2"))
	require.NoError(t, err)
	assert.False(t, received.Duplicate)
	assert.True(t, received.Call.Verified)
	got, err := r.GetCall(ctx, received.Call.ID)
	require.NoError(t, err)
	assert.Equal(t, received.Call, got)

	// No webhook has a token which isn't one
	_, err = r.Call(ctx, "not-a-token", githubRequest(body, "s3cret", "d4"))
	assert.ErrorIs(t, err, llm.ErrNotFound)
}

func Test_Registry_CallWithoutSecret(t *testing.T) {
	ctx := context.Background()
	r, _ := newRegistry(t)
	hook, err := r.CreateWebhook(ctx, schema.WebhookMeta{Session: uuid.New(), Description: "Generic"})
	require.NoError(t, err)

	// A webhook without a secret keeps its calls, which aren't verified
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("a=1&b=2"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	received, err := r.Call(ctx, hook.Token, req)
	require.NoError(t, err)
	assert.False(t, received.Call.Verified)
	assert.JSONEq(t, `{"a":"1","b":"2"}`, string(received.Call.Values))
	assert.Empty(t, received.Call.Delivery)

	// Calls without a delivery are each kept, and a GET is a call too
	received, err = r.Call(ctx, hook.Token, httptest.NewRequest(http.MethodGet, "/?status=ok", nil))
	require.NoError(t, err)
	assert.NotZero(t, received.Call.ID)
	assert.Equal(t, http.MethodGet, received.Call.Method)
	assert.Nil(t, received.Call.Body)

	// A body which isn't what its content type says is refused
	req = httptest.NewRequest(http.MethodPost, "/", strings.NewReader("{"))
	req.Header.Set("Content-Type", "application/json")
	_, err = r.Call(ctx, hook.Token, req)
	assert.ErrorIs(t, err, llm.ErrBadParameter)
	list, err := r.ListCalls(ctx, schema.CallListRequest{Webhook: hook.ID, Status: schema.CallRefused})
	require.NoError(t, err)
	require.Len(t, list.Body, 1)
	assert.Contains(t, list.Body[0].Error, "JSON")
	assert.Equal(t, "{", string(list.Body[0].Body))
}

func Test_Registry_CallMaxBody(t *testing.T) {
	ctx := context.Background()
	s, err := store.New(ctx, sqlite.Memory)
	require.NoError(t, err)
	defer s.Close()
	r, err := registry.New(s, registry.WithMaxBody(10))
	require.NoError(t, err)
	hook, err := r.CreateWebhook(ctx, schema.WebhookMeta{Session: uuid.New(), Description: "Generic"})
	require.NoError(t, err)

	// Up to the largest, but not beyond it, whether it says how large it is,
	// or not
	_, err = r.Call(ctx, hook.Token, httptest.NewRequest(http.MethodPost, "/", strings.NewReader("0123456789")))
	assert.NoError(t, err)
	_, err = r.Call(ctx, hook.Token, httptest.NewRequest(http.MethodPost, "/", strings.NewReader("0123456789a")))
	assert.ErrorIs(t, err, llm.ErrBadParameter)
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("0123456789a"))
	req.ContentLength = -1
	_, err = r.Call(ctx, hook.Token, req)
	assert.ErrorIs(t, err, llm.ErrBadParameter)

	// Those which are too large are refused, without their bodies
	list, err := r.ListCalls(ctx, schema.CallListRequest{Webhook: hook.ID, Status: schema.CallRefused})
	require.NoError(t, err)
	require.Len(t, list.Body, 2)
	assert.Contains(t, list.Body[0].Error, "larger than 10 bytes")
	assert.Nil(t, list.Body[0].Body)

	_, err = registry.New(s, registry.WithMaxBody(0))
	assert.ErrorIs(t, err, llm.ErrBadParameter)
}
