package httpclient_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"uuid"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	eliza "github.com/mutablelogic/go-llm/provider/eliza"
	providerschema "github.com/mutablelogic/go-llm/provider/schema"
	registry "github.com/mutablelogic/go-llm/registry"
	httpclient "github.com/mutablelogic/go-llm/registry/httpclient"
	schema "github.com/mutablelogic/go-llm/registry/schema"
	test "github.com/mutablelogic/go-llm/registry/test"
	types "github.com/mutablelogic/go-server/pkg/types"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

// newSessionClient returns a client of a test server of a registry with a
// session store, or without one, whose requests and responses are checked
// against the API's OpenAPI specification when the test ends
func newSessionClient(t *testing.T, store bool) *httpclient.Client {
	t.Helper()
	var opts []registry.Opt
	if store {
		opts = append(opts, registry.WithSessionStore(test.NewSessionStore()), registry.WithToolkit(test.NewToolkit(t)))
	}
	provider, err := eliza.New()
	require.NoError(t, err)
	server := test.NewServer(t, test.NewRegistry(t, []llm.Provider{provider}, opts...))
	client, err := httpclient.New(server.URL + test.Prefix)
	require.NoError(t, err)
	return client
}

func Test_Sessions(t *testing.T) {
	ctx := context.Background()
	client := newSessionClient(t, true)
	meta := schema.SessionMeta{GeneratorMeta: schema.GeneratorMeta{
		Model:  types.Ptr(eliza.ModelName),
		System: types.Ptr("Be brief"),
		Tools:  []string{"weather:*"},
		Opt:    map[string]json.RawMessage{"temperature": json.RawMessage(`0.5`), "thinking": json.RawMessage(`false`)},
		Schema: json.RawMessage(`{"type":"string"}`),
	}}

	// Create, and get
	created, err := client.CreateSession(ctx, meta)
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil(), created.ID)
	assert.Equal(t, "eliza:eliza-1966-en", types.Value(created.Model))
	got, err := client.GetSession(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, created.ID, got.ID)
	assert.Equal(t, meta.Tools, got.Tools)

	// List, from an offset, and at most a limit of them
	_, err = client.CreateSession(ctx, schema.SessionMeta{})
	require.NoError(t, err)
	list, err := client.ListSessions(ctx, schema.SessionListRequest{})
	require.NoError(t, err)
	assert.Equal(t, uint(2), list.Count)
	assert.Len(t, list.Body, 2)
	page, err := client.ListSessions(ctx, schema.SessionListRequest{Offset: 1, Limit: types.Ptr(uint(1))})
	require.NoError(t, err)
	assert.Equal(t, uint(2), page.Count)
	require.Len(t, page.Body, 1)
	assert.Equal(t, list.Body[1].ID, page.Body[0].ID)

	// Update, which clears the system prompt, tools and schema, and removes
	// an option, keeping the rest
	updated, err := client.UpdateSession(ctx, created.ID, schema.SessionMeta{GeneratorMeta: schema.GeneratorMeta{
		System: types.Ptr(""),
		Tools:  []string{},
		Opt:    map[string]json.RawMessage{"thinking": json.RawMessage(`null`)},
		Schema: json.RawMessage(`null`),
	}})
	require.NoError(t, err)
	assert.Equal(t, created.Model, updated.Model)
	assert.Nil(t, updated.System)
	assert.Empty(t, updated.Tools)
	assert.Empty(t, updated.Schema)
	assert.Len(t, updated.Opt, 1)
	assert.JSONEq(t, `0.5`, string(updated.Opt["temperature"]))

	// Status
	typing, err := client.SetSessionStatus(ctx, created.ID, schema.SessionTyping)
	require.NoError(t, err)
	assert.Equal(t, schema.SessionTyping, typing.Status)
	_, err = client.SetSessionStatus(ctx, created.ID, schema.SessionReplying)
	assert.Equal(t, http.StatusBadRequest, status(t, err))

	// Delete
	deleted, err := client.DeleteSession(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, created.ID, deleted.ID)
	_, err = client.GetSession(ctx, created.ID)
	assert.Equal(t, http.StatusNotFound, status(t, err))
	_, err = client.UpdateSession(ctx, created.ID, meta)
	assert.Equal(t, http.StatusNotFound, status(t, err))
	_, err = client.UpdateSession(ctx, uuid.New(), schema.SessionMeta{})
	assert.Equal(t, http.StatusBadRequest, status(t, err))
}

func Test_SessionsNoStore(t *testing.T) {
	// A server without a session store has no sessions' API
	client := newSessionClient(t, false)
	_, err := client.ListSessions(context.Background(), schema.SessionListRequest{})
	assert.ErrorContains(t, err, "404")
}

func Test_Chat(t *testing.T) {
	// The question, and the reply, are added to the session, with and
	// without streaming
	ctx := context.Background()
	client := newSessionClient(t, true)
	session, err := client.CreateSession(ctx, schema.SessionMeta{GeneratorMeta: schema.GeneratorMeta{Model: types.Ptr(eliza.ModelName)}})
	require.NoError(t, err)
	question := func(text string) schema.ChatRequest {
		return schema.ChatRequest{Messages: []providerschema.MessageMeta{{Sender: "user", Content: []providerschema.ContentBlock{{Text: types.Ptr(text)}}}}}
	}

	response, err := client.Chat(ctx, session.ID, question("Hello"), nil)
	require.NoError(t, err)
	assert.Equal(t, session.ID, response.ID)
	require.Len(t, response.Messages, 2)
	reply := response.Messages[1].Text()
	assert.NotEmpty(t, reply)

	var streamed string
	response, err = client.Chat(ctx, session.ID, question("I feel fine"), func(_ llm.StreamType, text string) { streamed += text })
	require.NoError(t, err)
	assert.Equal(t, uint64(3), response.Serial)
	require.Len(t, response.Messages, 2)
	assert.Equal(t, response.Messages[1].Text(), strings.TrimSpace(streamed))

	got, err := client.GetSession(ctx, session.ID)
	require.NoError(t, err)
	assert.Empty(t, got.Messages)
	messages, err := client.ListSessionMessages(ctx, session.ID, schema.MessageListRequest{})
	require.NoError(t, err)
	assert.Equal(t, uint(4), messages.Count)
	replies, err := client.ListSessionMessages(ctx, session.ID, schema.MessageListRequest{Role: "assistant", Limit: types.Ptr(uint(1))})
	require.NoError(t, err)
	assert.Equal(t, uint(2), replies.Count)
	require.Len(t, replies.Body, 1)
	assert.Equal(t, reply, replies.Body[0].Text())

	// Errors
	_, err = client.Chat(ctx, session.ID, schema.ChatRequest{}, nil)
	assert.ErrorIs(t, err, llm.ErrBadParameter)
	_, err = client.Chat(ctx, uuid.New(), question("Hello"), nil)
	assert.Equal(t, http.StatusNotFound, status(t, err))
}
