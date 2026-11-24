package httpclient_test

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"
	"testing"
	"time"
	"uuid"

	// Packages
	test "github.com/mutablelogic/go-llm/registry/test"
	httpclient "github.com/mutablelogic/go-llm/webhook/httpclient"
	httphandler "github.com/mutablelogic/go-llm/webhook/httphandler"
	registry "github.com/mutablelogic/go-llm/webhook/registry"
	schema "github.com/mutablelogic/go-llm/webhook/schema"
	store "github.com/mutablelogic/go-llm/webhook/store/sqlite"
	httprouter "github.com/mutablelogic/go-server/pkg/httprouter"
	types "github.com/mutablelogic/go-server/pkg/types"
	sqlite "github.com/mutablelogic/go-sqlite"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

func Test_Client(t *testing.T) {
	ctx := t.Context()
	s, err := store.New(ctx, sqlite.Memory)
	require.NoError(t, err)
	defer s.Close()
	r, err := registry.New(s)
	require.NoError(t, err)
	server := test.NewServerFunc(t, func(router *httprouter.Router) error {
		return httphandler.RegisterHandlers(r, router, func() string { return "https://llm.example.com/api" })
	})
	c, err := httpclient.New(server.URL + test.Prefix)
	require.NoError(t, err)

	// Created, with its token, and URL
	session := uuid.New()
	created, err := c.CreateWebhook(ctx, schema.WebhookMeta{Session: session, Description: "GitHub", Type: "github"})
	require.NoError(t, err)
	assert.Equal(t, "https://llm.example.com/api/hook/"+created.Token, created.URL)
	assert.False(t, created.HasSecret)
	_, err = c.CreateWebhook(ctx, schema.WebhookMeta{Session: session, Description: "Unknown", Type: "unknown"})
	assert.Error(t, err)

	// Listed, got, and given its secret
	list, err := c.ListWebhooks(ctx, schema.WebhookListRequest{Session: session.String(), Limit: types.Ptr(uint64(10))})
	require.NoError(t, err)
	assert.Equal(t, uint(1), list.Count)
	got, err := c.GetWebhook(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, "GitHub", got.Description)
	got, err = c.UpdateWebhook(ctx, created.ID, schema.WebhookRequest{Secret: types.Ptr("s3cret")})
	require.NoError(t, err)
	assert.True(t, got.HasSecret)

	// Called, signed with its secret, and as a delivery which is sent
	// again, and refused, when it isn't signed
	body := `{"action":"opened"}`
	mac := hmac.New(sha256.New, []byte("s3cret"))
	mac.Write([]byte(body))
	header := http.Header{}
	header.Set("X-GitHub-Event", "issues")
	header.Set("X-GitHub-Delivery", "d1")
	header.Set("X-Hub-Signature-256", "sha256="+hex.EncodeToString(mac.Sum(nil)))
	response, err := c.Hook(ctx, created.Token, http.MethodPost, types.ContentTypeJSON, strings.NewReader(body), header)
	require.NoError(t, err)
	kept := response.Call
	assert.NotZero(t, kept)
	response, err = c.Hook(ctx, created.Token, http.MethodPost, types.ContentTypeJSON, strings.NewReader(body), header)
	require.NoError(t, err)
	assert.True(t, response.Duplicate)
	header.Del("X-Hub-Signature-256")
	_, err = c.Hook(ctx, created.Token, http.MethodPost, types.ContentTypeJSON, strings.NewReader(body), header)
	assert.Error(t, err)

	// Its calls, listed, by their status, and after one, and got
	calls, err := c.ListCalls(ctx, schema.CallListRequest{Webhook: created.ID})
	require.NoError(t, err)
	require.Equal(t, uint(2), calls.Count)
	first := calls.Body[0].ID
	calls, err = c.ListCalls(ctx, schema.CallListRequest{Webhook: created.ID, Newest: true})
	require.NoError(t, err)
	require.Len(t, calls.Body, 2)
	assert.Equal(t, first, calls.Body[1].ID)
	calls, err = c.ListCalls(ctx, schema.CallListRequest{Webhook: created.ID, Since: time.Now().Add(-time.Hour)})
	require.NoError(t, err)
	assert.Equal(t, uint(2), calls.Count)
	calls, err = c.ListCalls(ctx, schema.CallListRequest{Webhook: created.ID, Since: time.Now().Add(time.Hour)})
	require.NoError(t, err)
	assert.Zero(t, calls.Count)
	calls, err = c.ListCalls(ctx, schema.CallListRequest{Session: session.String(), Status: schema.CallRefused})
	require.NoError(t, err)
	require.Len(t, calls.Body, 1)
	assert.Contains(t, calls.Body[0].Error, "X-Hub-Signature-256")
	calls, err = c.ListCalls(ctx, schema.CallListRequest{After: kept, Status: schema.CallAccepted})
	require.NoError(t, err)
	assert.Zero(t, calls.Count)
	call, err := c.GetCall(ctx, kept)
	require.NoError(t, err)
	assert.Equal(t, "issues.opened", call.Event)
	assert.Equal(t, body, string(call.Body))
	_, err = c.GetCall(ctx, 999)
	assert.Error(t, err)

	// A test call is added
	test, err := c.TestCall(ctx, schema.TestRequest{Webhook: created.ID, Signature: schema.SignatureNone})
	require.NoError(t, err)
	assert.True(t, test.Test)
	_, err = c.TestCall(ctx, schema.TestRequest{Webhook: 999})
	assert.Error(t, err)
	_, err = c.TestCall(ctx, schema.TestRequest{Webhook: created.ID, Signature: "other"})
	assert.Error(t, err)

	// A call is deleted
	deleted, err := c.DeleteCall(ctx, kept)
	require.NoError(t, err)
	assert.Equal(t, call, deleted)
	_, err = c.DeleteCall(ctx, kept)
	assert.Error(t, err)

	// Deleted
	_, err = c.DeleteWebhook(ctx, created.ID)
	require.NoError(t, err)
	_, err = c.GetWebhook(ctx, created.ID)
	assert.Error(t, err)
	_, err = c.Hook(ctx, created.Token, http.MethodPost, types.ContentTypeJSON, strings.NewReader(body), nil)
	assert.Error(t, err)
}
