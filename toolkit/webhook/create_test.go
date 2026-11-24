package webhook

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"uuid"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	toolkit "github.com/mutablelogic/go-llm/toolkit"
	protocol "github.com/mutablelogic/go-llm/webhook/protocol"
	schema "github.com/mutablelogic/go-llm/webhook/schema"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

// newToolkit returns a toolkit, with the namespace of the server
func newToolkit(t *testing.T, s *server) *toolkit.Toolkit {
	t.Helper()
	ns, err := New(s.URL + "/api")
	require.NoError(t, err)
	tk, err := toolkit.New(t.Context())
	require.NoError(t, err)
	t.Cleanup(func() { tk.Close() })
	require.NoError(t, tk.Add(ns))
	return tk
}

// inSession returns the context, in the session
func inSession(ctx context.Context, session uuid.UUID) context.Context {
	return logSession(ctx, session, func(llm.StreamType, string) {})
}

// logSession returns the context, in the session, whose tools log to fn
func logSession(ctx context.Context, session uuid.UUID, fn llm.GeneratorStreamFn) context.Context {
	return toolkit.WithSession(ctx, toolkit.NewSession("create", session, fn, nil))
}

func Test_Create(t *testing.T) {
	s := newServer(t)
	s.endpoint = "https://webhook.example.com/api"
	tk, session := newToolkit(t, s), uuid.New()
	call := func(ctx context.Context, input string) (result createOutput, text string, err error) {
		data, err := tk.CallTool(ctx, "webhook:create", json.RawMessage(input))
		if err == nil {
			require.NoError(t, json.Unmarshal(data, &result))
		}
		return result, string(data), err
	}

	// Created, for the session it's called in, with its URL, which works
	created, text, err := call(inSession(t.Context(), session), `{"description":" GitHub issues ","type":"github"}`)
	require.NoError(t, err)
	assert.False(t, created.HasSecret)
	assert.Contains(t, created.Note, "no secret")
	assert.NotContains(t, text, session.String())
	assert.Equal(t, "GitHub issues", created.Description)
	assert.Equal(t, "github", created.Type)
	assert.NotEmpty(t, created.Created)
	assert.Empty(t, created.Token)
	assert.Contains(t, created.Note, "Register the URL with the service")
	assert.Contains(t, created.Note, "Don't give it to the person")
	webhook, err := s.registry.GetWebhook(t.Context(), created.ID)
	require.NoError(t, err)
	assert.Equal(t, session, webhook.Session)
	assert.False(t, webhook.HasSecret)
	token, ok := strings.CutPrefix(created.URL, "https://webhook.example.com/api/hook/")
	require.True(t, ok, created.URL)
	assert.Len(t, token, 43)

	// Generic, without a type
	created, _, err = call(inSession(t.Context(), session), `{"description":"Anything"}`)
	require.NoError(t, err)
	assert.Equal(t, "generic", created.Type)
	_, err = s.registry.Call(t.Context(), strings.TrimPrefix(created.URL, "https://webhook.example.com/api/hook/"), newRequest("x"))
	assert.NoError(t, err)

	// With a secret, which its calls are verified with, and which isn't
	// returned, or logged
	var log strings.Builder
	created, text, err = call(logSession(t.Context(), session, func(_ llm.StreamType, text string) { log.WriteString(text) }), `{"description":"Signed","type":"github","secret":"s3cret"}`)
	require.NoError(t, err)
	assert.True(t, created.HasSecret)
	assert.Contains(t, created.Note, "signed with the secret")
	assert.NotContains(t, created.Note, "no secret")
	assert.NotContains(t, text, "s3cret")
	assert.NotContains(t, log.String(), "s3cret")
	assert.Contains(t, log.String(), "Signed")
	unsigned := newRequest("{}")
	unsigned.Header.Set("X-GitHub-Event", "ping")
	_, err = s.registry.Call(t.Context(), strings.TrimPrefix(created.URL, "https://webhook.example.com/api/hook/"), unsigned)
	assert.ErrorIs(t, err, llm.ErrForbidden)

	// With its token, when the server has no public endpoint
	s.endpoint = ""
	created, _, err = call(inSession(t.Context(), session), `{"description":"Local"}`)
	require.NoError(t, err)
	assert.Empty(t, created.URL)
	assert.Len(t, created.Token, 43)
	assert.Contains(t, created.Note, "/hook/")
	list, err := s.registry.ListWebhooks(t.Context(), schema.WebhookListRequest{Session: session.String()})
	require.NoError(t, err)
	assert.Equal(t, uint(4), list.Count)

	// Not without a session, such as when it's called by the API, nor for
	// another session, which isn't its to choose
	_, _, err = call(t.Context(), `{"description":"No session"}`)
	assert.ErrorIs(t, err, llm.ErrBadParameter)
	assert.ErrorContains(t, err, "conversation")
	other := uuid.New()
	_, _, _ = call(inSession(t.Context(), session), `{"description":"Another's","session":"`+other.String()+`"}`)
	list, err = s.registry.ListWebhooks(t.Context(), schema.WebhookListRequest{Session: other.String()})
	require.NoError(t, err)
	assert.Zero(t, list.Count)

	// Nor when it isn't valid, or the server can't be reached
	for _, input := range []string{``, `{}`, `{"description":" "}`, `{"description":"x","type":"unknown"}`, `{"description":1}`} {
		_, _, err = call(inSession(t.Context(), session), input)
		assert.Error(t, err, input)
	}
	s.down.Store(true)
	_, _, err = call(inSession(t.Context(), session), `{"description":"x"}`)
	assert.Error(t, err)
}

func Test_CreateLog(t *testing.T) {
	// What it's for is logged, but not its URL or token
	s := newServer(t)
	s.endpoint = "https://webhook.example.com/api"
	var log strings.Builder
	ctx := logSession(t.Context(), uuid.New(), func(_ llm.StreamType, text string) { log.WriteString(text) })
	data, err := newToolkit(t, s).CallTool(ctx, "webhook:create", json.RawMessage(`{"description":"GitHub issues"}`))
	require.NoError(t, err)
	var created createOutput
	require.NoError(t, json.Unmarshal(data, &created))
	require.NotEmpty(t, created.URL)
	assert.Contains(t, log.String(), "GitHub issues")
	assert.NotContains(t, log.String(), created.URL[strings.LastIndex(created.URL, "/")+1:])
}

func Test_CreateTypes(t *testing.T) {
	// The types which a model can give are the server's protocols
	var input struct {
		Properties struct {
			Type    struct{ Enum []string }
			Session any
		}
		Required []string
	}
	data, err := json.Marshal((&create{}).InputSchema())
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(data, &input))
	assert.ElementsMatch(t, protocol.Names(), input.Properties.Type.Enum)
	assert.Nil(t, input.Properties.Session)
	assert.Equal(t, []string{"description"}, input.Required)
}
