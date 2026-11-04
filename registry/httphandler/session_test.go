package httphandler_test

import (
	"net/http"
	"testing"
	"uuid"

	// Packages
	"context"
	llm "github.com/mutablelogic/go-llm"
	eliza "github.com/mutablelogic/go-llm/provider/eliza"
	providerschema "github.com/mutablelogic/go-llm/provider/schema"
	registry "github.com/mutablelogic/go-llm/registry"
	schema "github.com/mutablelogic/go-llm/registry/schema"
	test "github.com/mutablelogic/go-llm/registry/test"
	types "github.com/mutablelogic/go-server/pkg/types"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

func Test_Sessions(t *testing.T) {
	store := test.NewSessionStore()
	provider, err := eliza.New()
	require.NoError(t, err)
	server := test.NewServer(t, test.NewRegistry(t, []llm.Provider{provider}, registry.WithSessionStore(store), registry.WithToolkit(test.NewToolkit(t))))
	model := "eliza:eliza-1966-en" // the canonical name of eliza.ModelName, which is an alias

	t.Run("Create", func(t *testing.T) {
		var session schema.Session
		status := do(t, server, http.MethodPost, "llm/session", map[string]any{
			"model":  eliza.ModelName,
			"system": "Be brief",
			"tools":  []string{"memory:*"},
			"opt":    map[string]any{"temperature": 0.5, "thinking": false},
		}, &session)
		require.Equal(t, http.StatusCreated, status)
		assert.NotEmpty(t, session.ID)
		assert.Equal(t, uint64(1), session.Serial)
		assert.Equal(t, model, types.Value(session.Model))
		assert.Equal(t, "Be brief", types.Value(session.System))
		assert.Equal(t, []string{"memory:*"}, session.Tools)
		assert.Len(t, session.Opt, 2)
		assert.Empty(t, session.Messages)
	})

	t.Run("CreateInvalid", func(t *testing.T) {
		assert.Equal(t, http.StatusNotFound, do(t, server, http.MethodPost, "llm/session", map[string]any{"model": "missing"}, nil))
		assert.Equal(t, http.StatusNotFound, do(t, server, http.MethodPost, "llm/session", map[string]any{"tools": []string{"null"}}, nil))
		assert.Equal(t, http.StatusBadRequest, do(t, server, http.MethodPost, "llm/session", map[string]any{"schema": map[string]any{"type": 1}}, nil))
		assert.Equal(t, http.StatusBadRequest, do(t, server, http.MethodPost, "llm/session", map[string]any{"opt": map[string]any{"system_prompt": "x"}}, nil))
	})

	t.Run("Get", func(t *testing.T) {
		var created, session schema.Session
		require.Equal(t, http.StatusCreated, do(t, server, http.MethodPost, "llm/session", map[string]any{"model": eliza.ModelName}, &created))

		// A session is got without its messages, which are listed, oldest
		// first, of a role, from an offset, and at most a limit of them
		stored, err := store.Get(context.Background(), created.ID)
		require.NoError(t, err)
		says := func(role, text string, tokens uint64) providerschema.Message {
			return providerschema.Message{MessageMeta: providerschema.MessageMeta{Sender: role, Content: []providerschema.ContentBlock{{Text: types.Ptr(text)}}}, Tokens: tokens}
		}
		stored.Messages = []providerschema.Message{says("user", "Hi", 4), says("assistant", "Hello", 5), says("user", "Again", 6), says("assistant", "Still here", 7)}
		_, err = store.Put(context.Background(), stored)
		require.NoError(t, err)

		require.Equal(t, http.StatusOK, do(t, server, http.MethodGet, "llm/session/"+created.ID.String(), nil, &session))
		assert.Equal(t, created.ID, session.ID)
		assert.Empty(t, session.Messages)

		var messages schema.MessageList
		require.Equal(t, http.StatusOK, do(t, server, http.MethodGet, "llm/session/"+created.ID.String()+"/messages", nil, &messages))
		assert.Equal(t, uint(4), messages.Count)
		require.Len(t, messages.Body, 4)
		assert.Equal(t, "Hi", messages.Body[0].Text())
		assert.Equal(t, uint64(4), messages.Body[0].Tokens)

		var replies schema.MessageList
		require.Equal(t, http.StatusOK, do(t, server, http.MethodGet, "llm/session/"+created.ID.String()+"/messages?role=assistant&offset=1&limit=1", nil, &replies))
		assert.Equal(t, uint(2), replies.Count)
		require.Len(t, replies.Body, 1)
		assert.Equal(t, "Still here", replies.Body[0].Text())

		assert.Equal(t, http.StatusNotFound, do(t, server, http.MethodGet, "llm/session/"+uuid.New().String()+"/messages", nil, nil))
		assert.Equal(t, http.StatusBadRequest, do(t, server, http.MethodGet, "llm/session/not-a-uuid/messages", nil, nil))
		assert.Equal(t, http.StatusBadRequest, do(t, server, http.MethodGet, "llm/session/"+created.ID.String()+"/messages?limit=many", nil, nil))

		assert.Equal(t, http.StatusNotFound, do(t, server, http.MethodGet, "llm/session/"+uuid.New().String(), nil, nil))
		assert.Equal(t, http.StatusBadRequest, do(t, server, http.MethodGet, "llm/session/missing", nil, nil))
	})

	t.Run("List", func(t *testing.T) {
		// Sessions are listed without their messages, from the offset, and
		// at most the limit of them
		var list schema.SessionList
		require.Equal(t, http.StatusOK, do(t, server, http.MethodGet, "llm/session", nil, &list))
		assert.Equal(t, uint(store.Len()), list.Count)
		require.Len(t, list.Body, store.Len())
		for _, session := range list.Body {
			assert.Empty(t, session.Messages)
		}

		var page schema.SessionList
		require.Equal(t, http.StatusOK, do(t, server, http.MethodGet, "llm/session?offset=1&limit=1", nil, &page))
		assert.Equal(t, list.Count, page.Count)
		require.Len(t, page.Body, 1)
		assert.Equal(t, list.Body[1].ID, page.Body[0].ID)

		assert.Equal(t, http.StatusBadRequest, do(t, server, http.MethodGet, "llm/session?limit=many", nil, nil))
	})

	t.Run("Update", func(t *testing.T) {
		// What the update has is changed, and the rest is kept: the system
		// prompt is cleared when it's empty, the schema with null, the tools
		// with an empty list, and an option is removed with null
		var created, session schema.Session
		require.Equal(t, http.StatusCreated, do(t, server, http.MethodPost, "llm/session", map[string]any{
			"model":  eliza.ModelName,
			"system": "Be brief",
			"tools":  []string{"weather:*"},
			"opt":    map[string]any{"temperature": 0.5, "thinking": false},
			"schema": map[string]any{"type": "string"},
		}, &created))

		require.Equal(t, http.StatusOK, do(t, server, http.MethodPatch, "llm/session/"+created.ID.String(), map[string]any{
			"opt": map[string]any{"temperature": 0.9, "thinking": nil},
		}, &session))
		assert.Equal(t, model, types.Value(session.Model))
		assert.Equal(t, "Be brief", types.Value(session.System))
		assert.Equal(t, []string{"weather:*"}, session.Tools)
		assert.JSONEq(t, `{"type":"string"}`, string(session.Schema))
		require.Len(t, session.Opt, 1)
		assert.JSONEq(t, `0.9`, string(session.Opt["temperature"]))
		assert.Greater(t, session.Serial, created.Serial)

		var cleared schema.Session
		require.Equal(t, http.StatusOK, do(t, server, http.MethodPatch, "llm/session/"+created.ID.String(), map[string]any{
			"model":  model,
			"system": "",
			"tools":  []string{},
			"schema": nil,
		}, &cleared))
		assert.Equal(t, model, types.Value(cleared.Model))
		assert.Nil(t, cleared.System)
		assert.Empty(t, cleared.Tools)
		assert.Empty(t, cleared.Schema)
		assert.Len(t, cleared.Opt, 1)

		assert.Equal(t, http.StatusNotFound, do(t, server, http.MethodPatch, "llm/session/"+uuid.New().String(), map[string]any{"model": "x"}, nil))
		assert.Equal(t, http.StatusBadRequest, do(t, server, http.MethodPatch, "llm/session/"+created.ID.String(), map[string]any{}, nil))
		assert.Equal(t, http.StatusBadRequest, do(t, server, http.MethodPatch, "llm/session/"+created.ID.String(), map[string]any{"opt": map[string]any{"json_schema": "x"}}, nil))
	})

	t.Run("Delete", func(t *testing.T) {
		var created, deleted schema.Session
		require.Equal(t, http.StatusCreated, do(t, server, http.MethodPost, "llm/session", map[string]any{"model": eliza.ModelName}, &created))
		require.Equal(t, http.StatusOK, do(t, server, http.MethodDelete, "llm/session/"+created.ID.String(), nil, &deleted))
		assert.Equal(t, created.ID, deleted.ID)
		assert.Equal(t, http.StatusNotFound, do(t, server, http.MethodGet, "llm/session/"+created.ID.String(), nil, nil))
		assert.Equal(t, http.StatusNotFound, do(t, server, http.MethodDelete, "llm/session/"+created.ID.String(), nil, nil))
		assert.Equal(t, http.StatusBadRequest, do(t, server, http.MethodDelete, "llm/session/1", nil, nil))
	})
}

func Test_SessionsNoStore(t *testing.T) {
	// A server without a session store has no sessions
	server, _ := newServer(t)
	assert.Equal(t, http.StatusServiceUnavailable, do(t, server, http.MethodGet, "llm/session", nil, nil))
	assert.Equal(t, http.StatusServiceUnavailable, do(t, server, http.MethodPost, "llm/session", map[string]any{"model": eliza.ModelName}, nil))
	assert.Equal(t, http.StatusServiceUnavailable, do(t, server, http.MethodGet, "llm/session/"+uuid.New().String(), nil, nil))
	assert.Equal(t, http.StatusServiceUnavailable, do(t, server, http.MethodPatch, "llm/session/"+uuid.New().String(), map[string]any{"model": eliza.ModelName}, nil))
	assert.Equal(t, http.StatusServiceUnavailable, do(t, server, http.MethodDelete, "llm/session/"+uuid.New().String(), nil, nil))
}

func Test_Chat(t *testing.T) {
	provider, err := eliza.New()
	require.NoError(t, err)
	server := test.NewServer(t, test.NewRegistry(t, []llm.Provider{provider}, registry.WithSessionStore(test.NewSessionStore())))
	question := func(text string) map[string]any {
		return map[string]any{"messages": []map[string]any{{"role": "user", "content": []map[string]any{{"text": text}}}}}
	}

	var session schema.Session
	require.Equal(t, http.StatusCreated, do(t, server, http.MethodPost, "llm/session", map[string]any{"model": eliza.ModelName}, &session))

	// The question, and the reply, are added to the session
	var response schema.ChatResponse
	require.Equal(t, http.StatusOK, do(t, server, http.MethodPost, "llm/session/"+session.ID.String(), question("Hello"), &response))
	assert.Equal(t, session.ID, response.ID)
	assert.Equal(t, uint64(2), response.Serial)
	require.Len(t, response.Messages, 2)
	assert.Equal(t, "assistant", response.Messages[1].Role())
	assert.NotEmpty(t, response.Messages[1].Text())

	var got schema.Session
	require.Equal(t, http.StatusOK, do(t, server, http.MethodGet, "llm/session/"+session.ID.String(), nil, &got))
	assert.NotZero(t, got.Usage.Output)
	var messages schema.MessageList
	require.Equal(t, http.StatusOK, do(t, server, http.MethodGet, "llm/session/"+session.ID.String()+"/messages", nil, &messages))
	assert.Equal(t, uint(2), messages.Count)

	// Errors
	assert.Equal(t, http.StatusNotFound, do(t, server, http.MethodPost, "llm/session/"+uuid.New().String(), question("Hello"), nil))
	assert.Equal(t, http.StatusBadRequest, do(t, server, http.MethodPost, "llm/session/not-a-uuid", question("Hello"), nil))
	var noModel schema.Session
	require.Equal(t, http.StatusCreated, do(t, server, http.MethodPost, "llm/session", map[string]any{}, &noModel))
	assert.Equal(t, http.StatusBadRequest, do(t, server, http.MethodPost, "llm/session/"+noModel.ID.String(), question("Hello"), nil))
}
