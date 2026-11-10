package registry_test

import (
	"context"
	"encoding/json"
	"fmt"
	resourcemanager "github.com/mutablelogic/go-llm/resource"
	resourceschema "github.com/mutablelogic/go-llm/resource/schema"
	"strings"
	"sync"
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
		return eventschema.Event{Action: action, Schema: registry.EventSchema, Table: toolkit.ToolTable, Row: registry.AgentNamespace + ":" + name}
	}

	t.Run("Changes", func(t *testing.T) {
		// An agent which is created is a tool, whose input and output are
		// its input and the schema of its reply, which is updated, and
		// deleted, as it is, and each is published
		r, tk, ch := newRegistry(t, test.NewAgentStore())
		_, _, err := r.CreateAgent(ctx, segment)
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

	t.Run("Timeout", func(t *testing.T) {
		// An agent, as a tool, has its timeout, or the time a tool has by
		// default, without one, and one which isn't a duration isn't saved
		r, tk, _ := newRegistry(t, test.NewAgentStore())
		_, _, err := r.CreateAgent(ctx, segment)
		require.NoError(t, err)
		timeout := func() time.Duration {
			t.Helper()
			list, err := tk.Tools(ctx, "agent:segment")
			require.NoError(t, err)
			require.Len(t, list, 1)
			return list[0].Timeout()
		}
		assert.Zero(t, timeout())
		_, err = r.UpdateAgent(ctx, "segment", schema.AgentMeta{PromptMeta: schema.PromptMeta{Timeout: types.Ptr("2m")}})
		require.NoError(t, err)
		assert.Equal(t, 2*time.Minute, timeout())
		_, err = r.UpdateAgent(ctx, "segment", schema.AgentMeta{PromptMeta: schema.PromptMeta{Timeout: types.Ptr("1d")}})
		require.NoError(t, err)
		assert.Equal(t, 24*time.Hour, timeout(), "in days, too")
		for _, bad := range []string{"soon", "-1m", "0s"} {
			_, err = r.UpdateAgent(ctx, "segment", schema.AgentMeta{PromptMeta: schema.PromptMeta{Timeout: types.Ptr(bad)}})
			assert.ErrorIs(t, err, llm.ErrBadParameter, bad)
		}
		_, err = r.UpdateAgent(ctx, "segment", schema.AgentMeta{PromptMeta: schema.PromptMeta{Timeout: types.Ptr("")}})
		require.NoError(t, err)
		assert.Zero(t, timeout(), "empty clears it")
	})

	t.Run("Run", func(t *testing.T) {
		// An agent, as a tool, which has no model, can't be run without a
		// session to run it from, and one without an input has an empty
		// object as its input
		r, tk, _ := newRegistry(t, test.NewAgentStore())
		_, _, err := r.CreateAgent(ctx, schema.AgentMeta{PromptMeta: schema.PromptMeta{Name: "empty"}})
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

	t.Run("Concurrent", func(t *testing.T) {
		// Agents which are changed at once end as their tools, as they're
		// stored, and one which is deleted at the end isn't a tool
		r, tk, _ := newRegistry(t, test.NewAgentStore())
		_, _, err := r.CreateAgent(ctx, segment)
		require.NoError(t, err)
		var wg sync.WaitGroup
		for i := range 20 {
			wg.Go(func() {
				_, err := r.UpdateAgent(ctx, "segment", schema.AgentMeta{PromptMeta: schema.PromptMeta{Description: types.Ptr(fmt.Sprint("Description ", i))}})
				assert.NoError(t, err)
			})
			wg.Go(func() {
				name := fmt.Sprint("other", i%3)
				if _, _, err := r.CreateAgent(ctx, schema.AgentMeta{PromptMeta: schema.PromptMeta{Name: name}}); err == nil {
					// It may have been deleted by another, which created it
					// as it is, at once
					if _, err = r.DeleteAgent(ctx, name); err != nil {
						assert.ErrorIs(t, err, llm.ErrNotFound)
					}
				}
			})
		}
		wg.Wait()
		agent, err := r.GetAgent(ctx, "segment")
		require.NoError(t, err)
		tool, err := tk.GetTool(ctx, "agent:segment")
		require.NoError(t, err)
		assert.Equal(t, types.Value(agent.Description), tool.Description)
		assert.Equal(t, []string{"agent:segment"}, tools(t, tk))
	})

	t.Run("NoToolkit", func(t *testing.T) {
		// Without a toolkit, agents aren't tools
		r, err := registry.New(registry.WithAgentStore(test.NewAgentStore()))
		require.NoError(t, err)
		_, _, err = r.CreateAgent(ctx, segment)
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
		_, _, err = r.CreateAgent(ctx, schema.AgentMeta{
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
		_, _, err = r.CreateAgent(ctx, schema.AgentMeta{PromptMeta: schema.PromptMeta{
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
		_, _, err := r.CreateAgent(ctx, schema.AgentMeta{
			PromptMeta:    schema.PromptMeta{Name: "sections", Prompt: types.Ptr("Sections")},
			GeneratorMeta: schema.GeneratorMeta{Model: types.Ptr("model"), Schema: json.RawMessage(`{"type":"object","properties":{"sections":{"type":"array"}}}`)},
		})
		require.NoError(t, err)
		response, err := r.RunAgent(ctx, "sections", schema.RunAgentRequest{}, nil)
		require.NoError(t, err)
		assert.JSONEq(t, `{"sections":[]}`, string(response.Reply))
	})

	t.Run("NullSchema", func(t *testing.T) {
		// An agent whose schema is null replies with text, as a JSON string
		r, _, _ := newRegistry(t, text("42"))
		_, _, err := r.CreateAgent(ctx, schema.AgentMeta{
			PromptMeta:    schema.PromptMeta{Name: "answer", Prompt: types.Ptr("Answer")},
			GeneratorMeta: schema.GeneratorMeta{Model: types.Ptr("model"), Schema: json.RawMessage(`null`)},
		})
		require.NoError(t, err)
		response, err := r.RunAgent(ctx, "answer", schema.RunAgentRequest{}, nil)
		require.NoError(t, err)
		assert.Equal(t, `"42"`, string(response.Reply))
	})

	t.Run("Unfinished", func(t *testing.T) {
		// An agent, as a tool, whose reply is cut short, is the tool's error
		truncated := text("Partly")
		truncated.Result = providerschema.ResultMaxTokens
		r, _, _ := newRegistry(t, truncated)
		_, _, err := r.CreateAgent(ctx, schema.AgentMeta{
			PromptMeta:    schema.PromptMeta{Name: "long", Prompt: types.Ptr("Write a lot")},
			GeneratorMeta: schema.GeneratorMeta{Model: types.Ptr("model")},
		})
		require.NoError(t, err)
		_, err = r.Toolkit().CallTool(ctx, "agent:long", nil)
		assert.ErrorIs(t, err, llm.ErrMaxTokens)
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
		var progress []string
		response, err := r.Chat(ctx, session.ID, []llm.Message{user(t, "Segment this")}, func(what llm.StreamType, text string) {
			if what == llm.StreamProgress {
				progress = append(progress, llm.ParseProgress(text).Message)
			}
		})
		require.NoError(t, err)
		assert.Equal(t, "Done", response.Messages[len(response.Messages)-1].Text())
		assert.Equal(t, []string{"Agent segment is replying"}, progress, "the agent's progress is streamed, as it replies")

		// The agent's tokens are its tool result's, and the chat's, which
		// asked the model twice, and the agent once
		assert.Equal(t, llm.Usage{InputTokens: 30, OutputTokens: 15}, withoutDuration(response.Usage.Usage))
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
		var usage *llm.Usage
		for _, message := range stored.Messages {
			for _, block := range message.Content {
				if block.ToolResult != nil {
					usage = block.ToolResult.Usage
				}
			}
		}
		require.NotNil(t, usage, "the tool result has the agent's tokens")
		assert.Equal(t, llm.Usage{InputTokens: 10, OutputTokens: 5}, withoutDuration(*usage))

		// The agent was asked with its prompt and system prompt, rather than
		// the session's conversation
		require.Len(t, p.asked, 3)
		require.Len(t, p.asked[1].messages, 1)
		assert.Equal(t, "Segment", p.asked[1].messages[0].Text())
		system, _ := llm.GetOpt[string](p.asked[1].opts, llm.SystemPromptKey)
		assert.Equal(t, "You segment transcripts", system)
	})
}

func Test_AgentResources(t *testing.T) {
	// An agent's input, of format resource, which is a file's URI, is the
	// file's text, as it is, in the session it's run from, or on its own,
	// which the model is sent, rather than the URI, and a file which isn't
	// text, or doesn't exist, is an error
	ctx := context.Background()
	sessions := test.NewSessionStore()
	manager, err := resourcemanager.New(resourcemanager.NewStore(), resourcemanager.WithSessions(func(ctx context.Context, id uuid.UUID) error {
		_, err := sessions.Get(ctx, id)
		return err
	}))
	require.NoError(t, err)
	tk, err := toolkit.New(ctx)
	require.NoError(t, err)
	r, err := registry.New(registry.WithToolkit(tk), registry.WithAgentStore(test.NewAgentStore()), registry.WithSessionStore(sessions), registry.WithResources(manager))
	require.NoError(t, err)
	p := newScripted(text("Two sections"), text("One section"), text("Plain"))
	require.NoError(t, r.Add("", p))
	_, _, err = r.CreateAgent(ctx, schema.AgentMeta{PromptMeta: schema.PromptMeta{
		Name:   "segment",
		Input:  json.RawMessage(`{"type":"object","properties":{"transcript":{"type":"string","format":"resource","description":"The transcript."},"title":{"type":"string"}},"required":["transcript"]}`),
		Prompt: types.Ptr("Segment {{ .title }}:\n{{ .transcript }}"),
	}, GeneratorMeta: schema.GeneratorMeta{Model: types.Ptr("model")}})
	require.NoError(t, err)
	session, err := r.CreateSession(ctx, schema.SessionMeta{GeneratorMeta: schema.GeneratorMeta{Model: types.Ptr("model")}})
	require.NoError(t, err)
	srt := "1\n00:00:00,200 --> 00:00:03,500\nHello.\n"
	subtitles, _, err := manager.Create(ctx, resourceschema.ResourceMeta{Session: &session.ID, Name: "talk.srt", Type: "application/x-subrip"}, strings.NewReader(srt))
	require.NoError(t, err)
	alone, _, err := manager.Create(ctx, resourceschema.ResourceMeta{Name: "notes.txt", Type: "text/plain"}, strings.NewReader("Notes"))
	require.NoError(t, err)
	image, _, err := manager.Create(ctx, resourceschema.ResourceMeta{Session: &session.ID, Name: "cat.png"}, strings.NewReader(pngData))
	require.NoError(t, err)
	run := func(input string) error {
		_, err := r.RunAgent(ctx, "segment", schema.RunAgentRequest{Session: session.ID, Input: json.RawMessage(input)}, nil)
		return err
	}

	// The session's file, as it is, and one on its own, and text, which is
	// as it is, as is a URI of a property which isn't of format resource
	require.NoError(t, run(`{"transcript":"`+subtitles.URI()+`","title":"`+image.URI()+`"}`))
	assert.Equal(t, "Segment "+image.URI()+":\n"+srt, p.asked[0].messages[0].Text())
	require.NoError(t, run(`{"transcript":"`+alone.URI()+`"}`))
	assert.Equal(t, "Segment :\nNotes", p.asked[1].messages[0].Text())
	require.NoError(t, run(`{"transcript":"file:///etc/passwd"}`))
	assert.Equal(t, "Segment :\nfile:///etc/passwd", p.asked[2].messages[0].Text(), "only resource URIs are read")

	// An image isn't text, and says which tool reads it, and a file which
	// doesn't exist isn't one
	err = run(`{"transcript":"` + image.URI() + `"}`)
	assert.ErrorIs(t, err, llm.ErrBadParameter)
	assert.ErrorContains(t, err, "llm:ocr")
	assert.ErrorContains(t, err, `input "transcript"`)
	err = run(`{"transcript":"resource://` + strings.Repeat("0", 64) + `"}`)
	assert.ErrorIs(t, err, llm.ErrNotFound)

	// The model which calls the agent is told it can give a file
	list, err := tk.ListTools(ctx, toolschema.ToolListRequest{Namespace: registry.AgentNamespace})
	require.NoError(t, err)
	require.Len(t, list.Body, 1)
	assert.Contains(t, string(list.Body[0].Input), "The transcript. It can be the text, or the URI of a file")

	// Called by a model, through the toolkit, which validates its input
	// with the tool's schema, a file's URI isn't held to the pattern of its
	// text, which the file's text is, as the agent runs, so subtitles are
	// read, and notes, which aren't subtitles, aren't
	_, _, err = r.CreateAgent(ctx, schema.AgentMeta{PromptMeta: schema.PromptMeta{
		Name:   "cues",
		Input:  json.RawMessage(`{"type":"object","properties":{"transcript":{"type":"string","format":"resource","pattern":"-->"}},"required":["transcript"]}`),
		Prompt: types.Ptr("{{ .transcript }}"),
	}, GeneratorMeta: schema.GeneratorMeta{Model: types.Ptr("model")}})
	require.NoError(t, err)
	tool, err := tk.GetTool(ctx, "agent:cues")
	require.NoError(t, err)
	assert.NotContains(t, string(tool.Input), "pattern", "the tool's schema hasn't the pattern of its text")
	p.replies = append(p.replies, text("Sections"))
	inChat := toolkit.WithSession(ctx, toolkit.NewSession("agent", session.ID, func(llm.StreamType, string) {}, nil))
	_, err = tk.CallTool(inChat, "agent:cues", json.RawMessage(`{"transcript":"`+subtitles.URI()+`"}`))
	require.NoError(t, err)
	assert.Equal(t, srt, p.asked[len(p.asked)-1].messages[0].Text())
	_, err = tk.CallTool(inChat, "agent:cues", json.RawMessage(`{"transcript":"`+alone.URI()+`"}`))
	assert.ErrorIs(t, err, llm.ErrBadParameter)
	assert.ErrorContains(t, err, "-->")
	_, err = tk.CallTool(inChat, "agent:cues", json.RawMessage(`{"transcript":"Text, which isn't subtitles"}`))
	assert.ErrorIs(t, err, llm.ErrBadParameter)
}

func Test_AgentNamespaces(t *testing.T) {
	// An agent which is built in is a tool in the builtin namespace, and one
	// which isn't in the agent namespace, and one which becomes built in,
	// as an older server's copy of a built-in agent does, moves from one to
	// the other
	ctx := t.Context()
	tk, err := toolkit.New(ctx)
	require.NoError(t, err)
	r, err := registry.New(registry.WithToolkit(tk), registry.WithAgentStore(test.NewAgentStore()))
	require.NoError(t, err)
	_, _, err = r.CreateAgent(ctx, schema.AgentMeta{PromptMeta: schema.PromptMeta{Name: "translate", Prompt: types.Ptr("Translate")}})
	require.NoError(t, err)
	_, err = tk.ToolNames(ctx, "agent:translate")
	require.NoError(t, err)

	_, _, err = r.CreateAgent(ctx, schema.AgentMeta{PromptMeta: schema.PromptMeta{Name: "translate", Prompt: types.Ptr("Translate"), Builtin: types.Ptr(true)}})
	require.NoError(t, err)
	_, err = tk.ToolNames(ctx, "builtin:translate")
	assert.NoError(t, err)
	_, err = tk.ToolNames(ctx, "agent:translate")
	assert.ErrorIs(t, err, llm.ErrNotFound)

	// It runs by its name, or its name in its namespace, but not another's
	got, err := r.GetAgent(ctx, "builtin:translate")
	require.NoError(t, err)
	assert.Equal(t, schema.BuiltinNamespace, got.Namespace())
	_, err = r.GetAgent(ctx, "agent:translate")
	assert.ErrorIs(t, err, llm.ErrNotFound)
	_, err = r.DeleteAgent(ctx, "builtin:translate")
	assert.ErrorIs(t, err, llm.ErrForbidden)
}
