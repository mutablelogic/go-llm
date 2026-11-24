package sqlite_test

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"net/url"
	"testing"
	"time"
	"uuid"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	webhook "github.com/mutablelogic/go-llm/webhook/schema"
	store "github.com/mutablelogic/go-llm/webhook/store/sqlite"
	types "github.com/mutablelogic/go-server/pkg/types"
	sqlite "github.com/mutablelogic/go-sqlite"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

func newStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.New(context.Background(), sqlite.Memory)
	require.NoError(t, err)
	t.Cleanup(func() { s.Close() })
	return s
}

func hash(token string) []byte {
	h := sha256.Sum256([]byte(token))
	return h[:]
}

func Test_Store_Webhooks(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	session, other := uuid.New(), uuid.New()

	// Created, with serial numbers, and its secret, but not its hash
	a, err := s.CreateWebhook(ctx, webhook.WebhookMeta{Session: session, Description: " GitHub issues ", Type: "GitHub", Secret: "s3cret"}, hash("a"))
	require.NoError(t, err)
	assert.NotZero(t, a.ID)
	assert.Equal(t, session, a.Session)
	assert.Equal(t, "GitHub issues", a.Description)
	assert.Equal(t, "github", a.Type)
	assert.Equal(t, "s3cret", a.Secret)
	assert.WithinDuration(t, time.Now(), a.Created, time.Minute)
	b, err := s.CreateWebhook(ctx, webhook.WebhookMeta{Session: session, Description: "Pidgeon"}, hash("b"))
	require.NoError(t, err)
	assert.Greater(t, b.ID, a.ID)
	assert.Empty(t, b.Secret)
	c, err := s.CreateWebhook(ctx, webhook.WebhookMeta{Session: other, Description: "Other"}, hash("c"))
	require.NoError(t, err)

	// Not, with a token's hash which another has, or without a session, or
	// a hash
	_, err = s.CreateWebhook(ctx, webhook.WebhookMeta{Session: session}, hash("a"))
	assert.ErrorIs(t, err, llm.ErrConflict)
	_, err = s.CreateWebhook(ctx, webhook.WebhookMeta{}, hash("d"))
	assert.ErrorIs(t, err, llm.ErrBadParameter)
	_, err = s.CreateWebhook(ctx, webhook.WebhookMeta{Session: session}, nil)
	assert.ErrorIs(t, err, llm.ErrBadParameter)

	// Got, by its serial number, or its token's hash
	got, err := s.GetWebhook(ctx, a.ID)
	require.NoError(t, err)
	assert.Equal(t, a, got)
	got, err = s.GetWebhookByHash(ctx, hash("b"))
	require.NoError(t, err)
	assert.Equal(t, b, got)
	_, err = s.GetWebhook(ctx, 999)
	assert.ErrorIs(t, err, llm.ErrNotFound)
	_, err = s.GetWebhookByHash(ctx, hash("missing"))
	assert.ErrorIs(t, err, llm.ErrNotFound)

	// Listed, newest first, of a session, with a limit
	list, err := s.ListWebhooks(ctx, webhook.WebhookListRequest{})
	require.NoError(t, err)
	assert.Equal(t, uint(3), list.Count)
	assert.Equal(t, []uint64{c.ID, b.ID, a.ID}, ids(list.Body))
	list, err = s.ListWebhooks(ctx, webhook.WebhookListRequest{Session: session.String(), Limit: types.Ptr(uint64(1))})
	require.NoError(t, err)
	assert.Equal(t, uint(2), list.Count)
	assert.Equal(t, []uint64{b.ID}, ids(list.Body))
	_, err = s.ListWebhooks(ctx, webhook.WebhookListRequest{Session: "not-a-session"})
	assert.ErrorIs(t, err, llm.ErrBadParameter)

	// Updated: what's set, and a secret which is removed, when it's empty
	b, err = s.UpdateWebhook(ctx, b.ID, webhook.WebhookRequest{Secret: types.Ptr("whsec_1")})
	require.NoError(t, err)
	assert.Equal(t, "whsec_1", b.Secret)
	assert.Equal(t, "Pidgeon", b.Description)
	b, err = s.UpdateWebhook(ctx, b.ID, webhook.WebhookRequest{Description: types.Ptr("Pidgeon inbox"), Secret: types.Ptr("")})
	require.NoError(t, err)
	assert.Empty(t, b.Secret)
	assert.Equal(t, "Pidgeon inbox", b.Description)
	_, err = s.UpdateWebhook(ctx, 999, webhook.WebhookRequest{})
	assert.ErrorIs(t, err, llm.ErrNotFound)

	// Deleted, and its serial number isn't used again
	deleted, err := s.DeleteWebhook(ctx, c.ID)
	require.NoError(t, err)
	assert.Equal(t, c.ID, deleted.ID)
	_, err = s.DeleteWebhook(ctx, c.ID)
	assert.ErrorIs(t, err, llm.ErrNotFound)
	d, err := s.CreateWebhook(ctx, webhook.WebhookMeta{Session: other}, hash("d"))
	require.NoError(t, err)
	assert.Greater(t, d.ID, c.ID)
}

func Test_Store_Calls(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	session := uuid.New()
	a, err := s.CreateWebhook(ctx, webhook.WebhookMeta{Session: session, Type: "github"}, hash("a"))
	require.NoError(t, err)
	b, err := s.CreateWebhook(ctx, webhook.WebhookMeta{Session: uuid.New()}, hash("b"))
	require.NoError(t, err)

	// Created, as it was received
	call := webhook.Call{
		Webhook:  a.ID,
		Method:   "POST",
		Query:    url.Values{"a": {"1", "2"}},
		Headers:  map[string]string{"Content-Type": "application/json", "X-Github-Event": "issues"},
		Values:   json.RawMessage(`{"action":"opened"}`),
		Event:    "issues.opened",
		Delivery: "d1",
		Verified: true,
		Body:     []byte(`{"action":"opened"}`),
	}
	one, err := s.CreateCall(ctx, call)
	require.NoError(t, err)
	assert.NotZero(t, one.ID)
	assert.WithinDuration(t, time.Now(), one.Received, time.Minute)
	assert.Equal(t, call.Query, one.Query)
	assert.Equal(t, call.Headers, one.Headers)
	assert.JSONEq(t, string(call.Values), string(one.Values))
	assert.Equal(t, "issues.opened", one.Event)
	assert.Equal(t, "d1", one.Delivery)
	assert.True(t, one.Verified)
	assert.Equal(t, call.Body, one.Body)
	got, err := s.GetCall(ctx, one.ID)
	require.NoError(t, err)
	assert.Equal(t, one, got)

	// A delivery which is sent again is a conflict, and a call without one,
	// or of another webhook, isn't
	_, err = s.CreateCall(ctx, call)
	assert.ErrorIs(t, err, llm.ErrConflict)
	two, err := s.CreateCall(ctx, webhook.Call{Webhook: a.ID, Method: "GET"})
	require.NoError(t, err)
	assert.Nil(t, two.Query)
	assert.Nil(t, two.Headers)
	assert.Nil(t, two.Values)
	assert.Nil(t, two.Body)
	assert.False(t, two.Verified)
	three, err := s.CreateCall(ctx, webhook.Call{Webhook: b.ID, Method: "POST", Delivery: "d1"})
	require.NoError(t, err)

	// Not, of a webhook which isn't there, or without a method
	_, err = s.CreateCall(ctx, webhook.Call{Webhook: 999, Method: "POST"})
	assert.ErrorIs(t, err, llm.ErrNotFound)
	_, err = s.CreateCall(ctx, webhook.Call{Webhook: a.ID})
	assert.ErrorIs(t, err, llm.ErrBadParameter)

	// Listed, oldest first, after a call, of a webhook, or a session
	list, err := s.ListCalls(ctx, webhook.CallListRequest{})
	require.NoError(t, err)
	assert.Equal(t, uint(3), list.Count)
	assert.Equal(t, []uint64{one.ID, two.ID, three.ID}, callIDs(list.Body))
	list, err = s.ListCalls(ctx, webhook.CallListRequest{After: one.ID, Limit: types.Ptr(uint64(1))})
	require.NoError(t, err)
	assert.Equal(t, uint(2), list.Count)
	assert.Equal(t, []uint64{two.ID}, callIDs(list.Body))
	list, err = s.ListCalls(ctx, webhook.CallListRequest{Webhook: b.ID})
	require.NoError(t, err)
	assert.Equal(t, []uint64{three.ID}, callIDs(list.Body))
	list, err = s.ListCalls(ctx, webhook.CallListRequest{Session: session.String()})
	require.NoError(t, err)
	assert.Equal(t, []uint64{one.ID, two.ID}, callIDs(list.Body))

	// A call which was refused is kept, but isn't a delivery which is sent
	// again, and is listed by its status
	refused, err := s.CreateCall(ctx, webhook.Call{Webhook: a.ID, Method: "POST", Delivery: "d2", Error: "signature isn't the webhook's"})
	require.NoError(t, err)
	assert.Equal(t, "signature isn't the webhook's", refused.Error)
	accepted, err := s.CreateCall(ctx, webhook.Call{Webhook: a.ID, Method: "POST", Delivery: "d2", Verified: true})
	require.NoError(t, err)
	list, err = s.ListCalls(ctx, webhook.CallListRequest{Webhook: a.ID, Status: webhook.CallAccepted})
	require.NoError(t, err)
	assert.Equal(t, []uint64{one.ID, two.ID, accepted.ID}, callIDs(list.Body))
	list, err = s.ListCalls(ctx, webhook.CallListRequest{Webhook: a.ID, Status: " Refused "})
	require.NoError(t, err)
	assert.Equal(t, []uint64{refused.ID}, callIDs(list.Body))
	list, err = s.ListCalls(ctx, webhook.CallListRequest{Webhook: a.ID, After: two.ID})
	require.NoError(t, err)
	assert.Equal(t, []uint64{refused.ID, accepted.ID}, callIDs(list.Body))
	_, err = s.ListCalls(ctx, webhook.CallListRequest{Status: "pending"})
	assert.ErrorIs(t, err, llm.ErrBadParameter)

	// Deleted, before a time, and with their webhook
	n, err := s.DeleteCalls(ctx, time.Now().Add(-time.Hour))
	require.NoError(t, err)
	assert.Zero(t, n)
	_, err = s.DeleteWebhook(ctx, b.ID)
	require.NoError(t, err)
	_, err = s.GetCall(ctx, three.ID)
	assert.ErrorIs(t, err, llm.ErrNotFound)
	n, err = s.DeleteCalls(ctx, time.Now().Add(time.Minute))
	require.NoError(t, err)
	assert.Equal(t, uint64(4), n)
	list, err = s.ListCalls(ctx, webhook.CallListRequest{})
	require.NoError(t, err)
	assert.Zero(t, list.Count)
}

func ids(webhooks []webhook.Webhook) []uint64 {
	var result []uint64
	for _, w := range webhooks {
		result = append(result, w.ID)
	}
	return result
}

func callIDs(calls []webhook.Call) []uint64 {
	var result []uint64
	for _, c := range calls {
		result = append(result, c.ID)
	}
	return result
}
