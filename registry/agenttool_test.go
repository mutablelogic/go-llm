package registry_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"
	"uuid"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	broadcaster "github.com/mutablelogic/go-llm/broadcaster"
	eventschema "github.com/mutablelogic/go-llm/broadcaster/schema"
	providerschema "github.com/mutablelogic/go-llm/provider/schema"
	registry "github.com/mutablelogic/go-llm/registry"
	schema "github.com/mutablelogic/go-llm/registry/schema"
	test "github.com/mutablelogic/go-llm/registry/test"
	toolkit "github.com/mutablelogic/go-llm/toolkit"
	toolschema "github.com/mutablelogic/go-llm/toolkit/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

func Test_AgentTools(t *testing.T) {
	ctx := context.Background()
	newRegistry := func(t *testing.T, store *test.AgentStore) (*registry.Registry, *toolkit.Toolkit, chan eventschema.Event) {
		t.Helper()
		b := broadcaster.New()
		tk, err := toolkit.New(ctx, toolkit.WithBroadcaster(b))
		require.NoError(t, err)
		r, err := registry.New(registry.WithBroadcaster(b), registry.WithToolkit(tk), registry.WithAgentStore(store))
		require.NoError(t, err)
		return r, tk, watch(t, r)
	}
	segment := schema.AgentMeta{PromptMeta: schema.PromptMeta{
		Name:        "segment",
		Title:       types.Ptr("Segment a transcript"),
		Description: types.Ptr("Segments a transcript into its sections"),
		Input:       json.RawMessage(`{"type":"object","properties":{"transcript":{"type":"string"}}}`),
	}, GeneratorMeta: schema.GeneratorMeta{Schema: json.RawMessage(`{"type":"object"}`)}}
	tools := func(t *testing.T, tk *toolkit.Toolkit) []string {
		t.Helper()
		list, err := tk.ListTools(ctx, toolschema.ToolListRequest{Namespace: registry.AgentNamespace})
		require.NoError(t, err)
		var names []string
		for _, tool := range list.Body {
			names = append(names, tool.Canonical)
		}
		return names
	}
	toolEvent := func(action eventschema.EventAction, name string) eventschema.Event {
		return eventschema.Event{Action: action, Table: toolkit.ToolTable, Row: registry.AgentNamespace + ":" + name}
	}

	t.Run("Changes", func(t *testing.T) {
		// An agent which is created is a tool, whose input and output are
		// its input and the schema of its reply, which is updated, and
		// deleted, as it is, and each is published
		r, tk, ch := newRegistry(t, test.NewAgentStore())
		_, err := r.CreateAgent(ctx, segment)
		require.NoError(t, err)
		assert.Equal(t, []string{"agent:segment"}, tools(t, tk))
		tool, err := tk.GetTool(ctx, "agent:segment")
		require.NoError(t, err)
		assert.Equal(t, "Segments a transcript into its sections", tool.Description)
		assert.JSONEq(t, `{"type":"object","properties":{"transcript":{"type":"string"}}}`, string(tool.Input))
		assert.JSONEq(t, `{"type":"object"}`, string(tool.Output))

		_, err = r.UpdateAgent(ctx, "segment", schema.AgentMeta{PromptMeta: schema.PromptMeta{Description: types.Ptr("")}})
		require.NoError(t, err)
		tool, err = tk.GetTool(ctx, "agent:segment")
		require.NoError(t, err)
		assert.Equal(t, "Segment a transcript", tool.Description, "its title, without a description")

		_, err = r.DeleteAgent(ctx, "segment")
		require.NoError(t, err)
		assert.Empty(t, tools(t, tk))
		assert.Equal(t, []eventschema.Event{
			toolEvent(eventschema.EventCreate, "segment"),
			toolEvent(eventschema.EventUpdate, "segment"),
			toolEvent(eventschema.EventDelete, "segment"),
		}, events(ch))
	})

	t.Run("Run", func(t *testing.T) {
		// An agent, as a tool, which has no model, can't be run without a
		// session to run it from, and one without an input has an empty
		// object as its input
		r, tk, _ := newRegistry(t, test.NewAgentStore())
		_, err := r.CreateAgent(ctx, schema.AgentMeta{PromptMeta: schema.PromptMeta{Name: "empty"}})
		require.NoError(t, err)
		tool, err := tk.GetTool(ctx, "agent:empty")
		require.NoError(t, err)
		assert.Equal(t, "The agent empty", tool.Description)
		var input map[string]any
		require.NoError(t, json.Unmarshal(tool.Input, &input))
		assert.Equal(t, "object", input["type"])
		_, err = tk.CallTool(ctx, "agent:empty", json.RawMessage(`{}`))
		assert.ErrorIs(t, err, llm.ErrBadParameter)
	})

	t.Run("Loaded", func(t *testing.T) {
		// The stored agents are tools when the registry runs, as when the
		// server restarts
		store := test.NewAgentStore()
		for _, name := range []string{"segment", "summarize"} {
			_, err := store.Create(ctx, schema.AgentMeta{PromptMeta: schema.PromptMeta{Name: name}})
			require.NoError(t, err)
		}
		r, tk, _ := newRegistry(t, store)
		assert.Empty(t, tools(t, tk))
		runCtx, stop := context.WithCancel(ctx)
		running := make(chan error, 1)
		go func() { running <- r.Run(runCtx, nil) }()
		require.Eventually(t, func() bool { return len(tools(t, tk)) == 2 }, 5*time.Second, time.Millisecond)
		assert.Equal(t, []string{"agent:segment", "agent:summarize"}, tools(t, tk))
		stop()
		require.NoError(t, <-running)
	})

	t.Run("NoToolkit", func(t *testing.T) {
		// Without a toolkit, agents aren't tools
		r, err := registry.New(registry.WithAgentStore(test.NewAgentStore()))
		require.NoError(t, err)
		_, err = r.CreateAgent(ctx, segment)
		assert.NoError(t, err)
	})

	t.Run("Conflict", func(t *testing.T) {
		// A toolkit which has an agent namespace already can't have the
		// registry's
		tk, err := toolkit.New(ctx)
		require.NoError(t, err)
		require.NoError(t, tk.Add(toolkit.NewNamespace(registry.AgentNamespace, "")))
		_, err = registry.New(registry.WithToolkit(tk), registry.WithAgentStore(test.NewAgentStore()))
		assert.ErrorIs(t, err, llm.ErrConflict)
	})
}

func Test_RunAgent(t *testing.T) {
	ctx := context.Background()
	newRegistry := func(t *testing.T, replies ...*providerschema.Message) (*registry.Registry, *test.SessionStore, *scripted) {
		t.Helper()
		tk, err := toolkit.New(ctx)
		require.NoError(t, err)
		sessions := test.NewSessionStore()
		r, err := registry.New(registry.WithToolkit(tk), registry.WithAgentStore(test.NewAgentStore()), registry.WithSessionStore(sessions))
		require.NoError(t, err)
		p := newScripted(replies...)
		require.NoError(t, r.Add("", p))
		_, err = r.CreateAgent(ctx, schema.AgentMeta{
			PromptMeta:    schema.PromptMeta{Name: "segment", Prompt: types.Ptr("Segment")},
			GeneratorMeta: schema.GeneratorMeta{System: types.Ptr("You segment transcripts")},
		})
		require.NoError(t, err)
		return r, sessions, p
	}

	t.Run("Run", func(t *testing.T) {
		// The agent's prompt, executed with the input, is sent to its model,
		// with its system prompt, and none of the session's history, and its
		// reply is returned, with the messages it added
		r, _, p := newRegistry(t, text("Hello"), text("Summary of Hello"))
		session, err := r.CreateSession(ctx, schema.SessionMeta{GeneratorMeta: schema.GeneratorMeta{Model: types.Ptr("model")}})
		require.NoError(t, err)
		_, err = r.Chat(ctx, session.ID, []llm.Message{user(t, "Earlier")}, nil)
		require.NoError(t, err)
		_, err = r.CreateAgent(ctx, schema.AgentMeta{PromptMeta: schema.PromptMeta{
			Name:   "summarize",
			Input:  json.RawMessage(`{"type":"object","properties":{"text":{"type":"string"}},"required":["text"]}`),
			Prompt: types.Ptr("Summarize {{ .text }}"),
		}})
		require.NoError(t, err)
		var streamed string
		response, err := r.RunAgent(ctx, "summarize", schema.RunAgentRequest{Session: session.ID, Input: json.RawMessage(`{"text":"Hello"}`)}, func(_ llm.StreamType, text string) { streamed += text })
		require.NoError(t, err)
		assert.Equal(t, "summarize", response.Name)
		assert.NotZero(t, response.Serial)
		assert.JSONEq(t, `"Summary of Hello"`, string(response.Reply))
		assert.Equal(t, "Summary of Hello", streamed)
		require.Len(t, response.Messages, 2)
		assert.Equal(t, "Summarize Hello", response.Messages[0].Text())
		assert.NotZero(t, response.Usage.InputTokens)
		require.Len(t, p.asked, 2)
		require.Len(t, p.asked[1].messages, 1, "without the session's history")
		assert.Equal(t, "Summarize Hello", p.asked[1].messages[0].Text())
	})

	t.Run("Schema", func(t *testing.T) {
		// The reply of an agent with a schema is what it replied, as JSON
		r, _, _ := newRegistry(t, call(toolkit.RespondName, `{"result":{"sections":[]}}`))
		_, err := r.CreateAgent(ctx, schema.AgentMeta{
			PromptMeta:    schema.PromptMeta{Name: "sections", Prompt: types.Ptr("Sections")},
			GeneratorMeta: schema.GeneratorMeta{Model: types.Ptr("model"), Schema: json.RawMessage(`{"type":"object","properties":{"sections":{"type":"array"}}}`)},
		})
		require.NoError(t, err)
		response, err := r.RunAgent(ctx, "sections", schema.RunAgentRequest{}, nil)
		require.NoError(t, err)
		assert.JSONEq(t, `{"sections":[]}`, string(response.Reply))
	})

	t.Run("Errors", func(t *testing.T) {
		// An agent without a model can't be run without a session, whose
		// model it's run with, and the input is validated first
		r, _, _ := newRegistry(t)
		_, err := r.RunAgent(ctx, "segment", schema.RunAgentRequest{}, nil)
		assert.ErrorIs(t, err, llm.ErrBadParameter)
		_, err = r.RunAgent(ctx, "segment", schema.RunAgentRequest{Session: uuid.New()}, nil)
		assert.ErrorIs(t, err, llm.ErrNotFound)
		_, err = r.RunAgent(ctx, "missing", schema.RunAgentRequest{}, nil)
		assert.ErrorIs(t, err, llm.ErrNotFound)
		_, err = r.RunAgent(ctx, "segment", schema.RunAgentRequest{Input: json.RawMessage(`"text"`)}, nil)
		assert.ErrorIs(t, err, llm.ErrBadParameter)
	})

	t.Run("FromSession", func(t *testing.T) {
		// An agent which a session's model calls, as a tool, is run from the
		// session, so with its model, and its reply is the tool's result
		r, sessions, p := newRegistry(t, call("agent__segment", `{}`), text("Segmented"), text("Done"))
		session, err := r.CreateSession(ctx, schema.SessionMeta{GeneratorMeta: schema.GeneratorMeta{Model: types.Ptr("model"), Tools: []string{"agent:*"}}})
		require.NoError(t, err)
		response, err := r.Chat(ctx, session.ID, []llm.Message{user(t, "Segment this")}, nil)
		require.NoError(t, err)
		assert.Equal(t, "Done", response.Messages[len(response.Messages)-1].Text())
		stored, err := sessions.Get(ctx, session.ID)
		require.NoError(t, err)
		var results []string
		for _, message := range stored.Messages {
			for _, result := range message.ToolResults() {
				results = append(results, string(result.Content))
			}
		}
		require.Len(t, results, 1)
		assert.JSONEq(t, `"Segmented"`, results[0])

		// The agent was asked with its prompt and system prompt, rather than
		// the session's conversation
		require.Len(t, p.asked, 3)
		require.Len(t, p.asked[1].messages, 1)
		assert.Equal(t, "Segment", p.asked[1].messages[0].Text())
		system, _ := llm.GetOpt[string](p.asked[1].opts, llm.SystemPromptKey)
		assert.Equal(t, "You segment transcripts", system)
	})
}
