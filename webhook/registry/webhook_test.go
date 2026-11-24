package registry_test

import (
	"context"
	"encoding/base64"
	"testing"
	"uuid"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	registry "github.com/mutablelogic/go-llm/webhook/registry"
	schema "github.com/mutablelogic/go-llm/webhook/schema"
	store "github.com/mutablelogic/go-llm/webhook/store/sqlite"
	types "github.com/mutablelogic/go-server/pkg/types"
	sqlite "github.com/mutablelogic/go-sqlite"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

func newRegistry(t *testing.T) (*registry.Registry, *store.Store) {
	t.Helper()
	s, err := store.New(context.Background(), sqlite.Memory)
	require.NoError(t, err)
	t.Cleanup(func() { s.Close() })
	r, err := registry.New(s)
	require.NoError(t, err)
	return r, s
}

func Test_Registry_New(t *testing.T) {
	_, err := registry.New(nil)
	assert.ErrorIs(t, err, llm.ErrBadParameter)
}

func Test_Registry_Webhooks(t *testing.T) {
	ctx := context.Background()
	r, s := newRegistry(t)
	session := uuid.New()

	// Created, with a token, which is random, and only its hash is kept,
	// and its type, which is a protocol, or generic
	a, err := r.CreateWebhook(ctx, schema.WebhookMeta{Session: session, Description: " GitHub ", Type: " GitHub ", Secret: "s3cret"})
	require.NoError(t, err)
	assert.Equal(t, "github", a.Type)
	assert.Equal(t, "GitHub", a.Description)
	assert.Empty(t, a.Secret)
	assert.True(t, a.HasSecret)
	token, err := base64.RawURLEncoding.DecodeString(a.Token)
	require.NoError(t, err)
	assert.Len(t, token, 32)
	stored, err := s.GetWebhookByHash(ctx, registry.Hash(a.Token))
	require.NoError(t, err)
	assert.Equal(t, a.ID, stored.ID)
	assert.Equal(t, "s3cret", stored.Secret, "the store keeps the secret, which calls are verified with")

	b, err := r.CreateWebhook(ctx, schema.WebhookMeta{Session: session, Description: "Pidgeon", Type: "pidgeon"})
	require.NoError(t, err)
	assert.False(t, b.HasSecret)
	assert.NotEqual(t, a.Token, b.Token)
	c, err := r.CreateWebhook(ctx, schema.WebhookMeta{Session: session, Description: "Generic"})
	require.NoError(t, err)
	assert.Equal(t, "generic", c.Type)

	// Not, without a session, or a description, or of a type which isn't a
	// protocol
	_, err = r.CreateWebhook(ctx, schema.WebhookMeta{Description: "No session"})
	assert.ErrorIs(t, err, llm.ErrBadParameter)
	_, err = r.CreateWebhook(ctx, schema.WebhookMeta{Session: session, Description: "  "})
	assert.ErrorIs(t, err, llm.ErrBadParameter)
	_, err = r.CreateWebhook(ctx, schema.WebhookMeta{Session: session, Description: "Unknown", Type: "unknown"})
	assert.ErrorIs(t, err, llm.ErrBadParameter)

	// Got, and listed, without their secrets
	got, err := r.GetWebhook(ctx, a.ID)
	require.NoError(t, err)
	assert.Equal(t, a.Webhook, got)
	list, err := r.ListWebhooks(ctx, schema.WebhookListRequest{Session: session.String()})
	require.NoError(t, err)
	assert.Equal(t, uint(3), list.Count)
	for _, w := range list.Body {
		assert.Empty(t, w.Secret)
	}
	assert.True(t, list.Body[2].HasSecret)
	_, err = r.GetWebhook(ctx, 999)
	assert.ErrorIs(t, err, llm.ErrNotFound)

	// Updated, such as with the secret a service gives, once it's
	// registered there
	b2, err := r.UpdateWebhook(ctx, b.ID, schema.WebhookRequest{Secret: types.Ptr("whsec_1")})
	require.NoError(t, err)
	b2, err = r.UpdateWebhook(ctx, b.ID, schema.WebhookRequest{Description: types.Ptr("  Pidgeon inbox ")})
	require.NoError(t, err)
	assert.Equal(t, "Pidgeon inbox", b2.Description)
	_, err = r.UpdateWebhook(ctx, b.ID, schema.WebhookRequest{Description: types.Ptr(" ")})
	assert.ErrorIs(t, err, llm.ErrBadParameter)
	assert.True(t, b2.HasSecret)
	assert.Empty(t, b2.Secret)

	// Deleted, so its token is no longer one
	deleted, err := r.DeleteWebhook(ctx, a.ID)
	require.NoError(t, err)
	assert.Equal(t, a.ID, deleted.ID)
	assert.Empty(t, deleted.Secret)
	_, err = s.GetWebhookByHash(ctx, registry.Hash(a.Token))
	assert.ErrorIs(t, err, llm.ErrNotFound)
	_, err = r.DeleteWebhook(ctx, a.ID)
	assert.ErrorIs(t, err, llm.ErrNotFound)
}
