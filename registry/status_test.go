package registry_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"
	"uuid"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	eventschema "github.com/mutablelogic/go-llm/broadcaster/schema"
	providerschema "github.com/mutablelogic/go-llm/provider/schema"
	registry "github.com/mutablelogic/go-llm/registry"
	schema "github.com/mutablelogic/go-llm/registry/schema"
	test "github.com/mutablelogic/go-llm/registry/test"
	toolkit "github.com/mutablelogic/go-llm/toolkit"
	types "github.com/mutablelogic/go-server/pkg/types"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

// sessionEvent returns the event of the session, with the data, when it
// isn't nil
func sessionEvent(t *testing.T, action eventschema.EventAction, id uuid.UUID, data *schema.SessionEvent) eventschema.Event {
	t.Helper()
	event := eventschema.Event{Action: action, Schema: registry.EventSchema, Table: registry.SessionTable, Row: id.String()}
	if data != nil {
		raw, err := json.Marshal(data)
		require.NoError(t, err)
		event.Data = raw
	}
	return event
}

// nextEvent returns the next event of the channel, or fails when there's
// none in time
func nextEvent(t *testing.T, ch <-chan eventschema.Event) eventschema.Event {
	t.Helper()
	select {
	case event := <-ch:
		return event
	case <-time.After(5 * time.Second):
		t.Fatal("no event")
		return eventschema.Event{}
	}
}

func Test_SessionEvents(t *testing.T) {
	ctx := context.Background()
	meta := schema.SessionMeta{GeneratorMeta: schema.GeneratorMeta{Model: types.Ptr("model")}}
	newRegistry := func(t *testing.T, replies ...string) (*registry.Registry, *test.SessionStore) {
		t.Helper()
		store := test.NewSessionStore()
		r, err := registry.New(registry.WithSessionStore(store))
		require.NoError(t, err)
		p := newScripted()
		for _, reply := range replies {
			p.replies = append(p.replies, text(reply))
		}
		require.NoError(t, r.Add("", p))
		return r, store
	}
	status := func(value schema.SessionStatus) *schema.SessionEvent {
		return &schema.SessionEvent{Status: value}
	}
	serial := func(value uint64) *schema.SessionEvent {
		return &schema.SessionEvent{Serial: value}
	}

	t.Run("Changes", func(t *testing.T) {
		// A session which is created, updated, chatted with and deleted is
		// published, with its serial, and while its model replies, it's
		// replying, until its reply is stored
		r, _ := newRegistry(t, "Hello")
		ch := watch(t, r)
		session, err := r.CreateSession(ctx, meta)
		require.NoError(t, err)
		assert.Equal(t, schema.SessionIdle, session.Status)
		_, err = r.UpdateSession(ctx, session.ID, schema.SessionMeta{GeneratorMeta: schema.GeneratorMeta{System: types.Ptr("Be brief")}})
		require.NoError(t, err)
		_, err = r.Chat(ctx, session.ID, []llm.Message{user(t, "Hi")}, nil)
		require.NoError(t, err)
		_, err = r.DeleteSession(ctx, session.ID)
		require.NoError(t, err)
		assert.Equal(t, []eventschema.Event{
			sessionEvent(t, eventschema.EventCreate, session.ID, serial(1)),
			sessionEvent(t, eventschema.EventUpdate, session.ID, serial(2)),
			sessionEvent(t, eventschema.EventUpdate, session.ID, status(schema.SessionReplying)),
			sessionEvent(t, eventschema.EventUpdate, session.ID, serial(3)),
			sessionEvent(t, eventschema.EventUpdate, session.ID, status(schema.SessionIdle)),
			sessionEvent(t, eventschema.EventDelete, session.ID, nil),
		}, events(ch))
	})

	t.Run("Replying", func(t *testing.T) {
		// A session is replying as its model replies, whatever it's set to,
		// and idle once it has, even when the reply isn't stored
		r, _ := newRegistry(t, "Hello")
		session, err := r.CreateSession(ctx, meta)
		require.NoError(t, err)
		ch := watch(t, r)
		_, err = r.Chat(ctx, session.ID, []llm.Message{user(t, "Hi")}, func(llm.StreamType, string) {
			got, err := r.GetSession(ctx, session.ID)
			require.NoError(t, err)
			assert.Equal(t, schema.SessionReplying, got.Status)
			got, err = r.SetSessionStatus(ctx, session.ID, schema.SessionTyping)
			require.NoError(t, err)
			assert.Equal(t, schema.SessionReplying, got.Status)
			_, err = r.DeleteSession(ctx, session.ID)
			assert.ErrorIs(t, err, llm.ErrConflict, "it isn't deleted as it's replying")
			_, err = r.UpdateSession(ctx, session.ID, schema.SessionMeta{GeneratorMeta: schema.GeneratorMeta{System: types.Ptr("Be brief")}})
			require.NoError(t, err)
		})
		assert.ErrorIs(t, err, llm.ErrConflict)
		got, err := r.GetSession(ctx, session.ID)
		require.NoError(t, err)
		assert.Equal(t, schema.SessionTyping, got.Status, "typing, as it was set as the model replied")
		assert.Equal(t, []eventschema.Event{
			sessionEvent(t, eventschema.EventUpdate, session.ID, status(schema.SessionReplying)),
			sessionEvent(t, eventschema.EventUpdate, session.ID, serial(2)),
			sessionEvent(t, eventschema.EventUpdate, session.ID, status(schema.SessionTyping)),
		}, events(ch))
	})

	t.Run("Typing", func(t *testing.T) {
		// A session is typing, as a client says, until it says it isn't, or
		// until TypingTimeout after it last said it is, and sending a
		// message ends it
		timeout := registry.TypingTimeout
		registry.TypingTimeout = 50 * time.Millisecond
		t.Cleanup(func() { registry.TypingTimeout = timeout })
		r, _ := newRegistry(t, "Hello")
		session, err := r.CreateSession(ctx, meta)
		require.NoError(t, err)
		ch := watch(t, r)

		got, err := r.SetSessionStatus(ctx, session.ID, schema.SessionTyping)
		require.NoError(t, err)
		assert.Equal(t, schema.SessionTyping, got.Status)
		_, err = r.SetSessionStatus(ctx, session.ID, schema.SessionTyping)
		require.NoError(t, err)
		_, err = r.SetSessionStatus(ctx, session.ID, schema.SessionIdle)
		require.NoError(t, err)
		assert.Equal(t, []eventschema.Event{
			sessionEvent(t, eventschema.EventUpdate, session.ID, status(schema.SessionTyping)),
			sessionEvent(t, eventschema.EventUpdate, session.ID, status(schema.SessionIdle)),
		}, events(ch), "typing again isn't a change")

		// It's idle after the timeout
		_, err = r.SetSessionStatus(ctx, session.ID, schema.SessionTyping)
		require.NoError(t, err)
		assert.Equal(t, sessionEvent(t, eventschema.EventUpdate, session.ID, status(schema.SessionTyping)), nextEvent(t, ch))
		assert.Equal(t, sessionEvent(t, eventschema.EventUpdate, session.ID, status(schema.SessionIdle)), nextEvent(t, ch))
		got, err = r.GetSession(ctx, session.ID)
		require.NoError(t, err)
		assert.Equal(t, schema.SessionIdle, got.Status)

		// And sending a message ends it
		_, err = r.SetSessionStatus(ctx, session.ID, schema.SessionTyping)
		require.NoError(t, err)
		_, err = r.Chat(ctx, session.ID, []llm.Message{user(t, "Hi")}, nil)
		require.NoError(t, err)
		assert.Equal(t, []eventschema.Event{
			sessionEvent(t, eventschema.EventUpdate, session.ID, status(schema.SessionTyping)),
			sessionEvent(t, eventschema.EventUpdate, session.ID, status(schema.SessionReplying)),
			sessionEvent(t, eventschema.EventUpdate, session.ID, serial(2)),
			sessionEvent(t, eventschema.EventUpdate, session.ID, status(schema.SessionIdle)),
		}, events(ch))
	})

	t.Run("Listed", func(t *testing.T) {
		// Sessions are listed with their statuses
		r, _ := newRegistry(t)
		session, err := r.CreateSession(ctx, meta)
		require.NoError(t, err)
		_, err = r.SetSessionStatus(ctx, session.ID, schema.SessionTyping)
		require.NoError(t, err)
		list, err := r.ListSessions(ctx, schema.SessionListRequest{})
		require.NoError(t, err)
		require.Len(t, list.Body, 1)
		assert.Equal(t, schema.SessionTyping, list.Body[0].Status)
	})

	t.Run("Deleted", func(t *testing.T) {
		// A session which is deleted is forgotten, so it's not published
		// as idle after it
		timeout := registry.TypingTimeout
		registry.TypingTimeout = 10 * time.Millisecond
		t.Cleanup(func() { registry.TypingTimeout = timeout })
		r, _ := newRegistry(t)
		session, err := r.CreateSession(ctx, meta)
		require.NoError(t, err)
		_, err = r.SetSessionStatus(ctx, session.ID, schema.SessionTyping)
		require.NoError(t, err)
		ch := watch(t, r)
		_, err = r.DeleteSession(ctx, session.ID)
		require.NoError(t, err)
		time.Sleep(50 * time.Millisecond)
		assert.Equal(t, []eventschema.Event{sessionEvent(t, eventschema.EventDelete, session.ID, nil)}, events(ch))
	})

	t.Run("Invalid", func(t *testing.T) {
		r, _ := newRegistry(t)
		session, err := r.CreateSession(ctx, meta)
		require.NoError(t, err)
		_, err = r.SetSessionStatus(ctx, session.ID, schema.SessionReplying)
		assert.ErrorIs(t, err, llm.ErrBadParameter)
		_, err = r.SetSessionStatus(ctx, session.ID, "")
		assert.ErrorIs(t, err, llm.ErrBadParameter)
		_, err = r.SetSessionStatus(ctx, uuid.New(), schema.SessionTyping)
		assert.ErrorIs(t, err, llm.ErrNotFound)
		r, err = registry.New()
		require.NoError(t, err)
		_, err = r.SetSessionStatus(ctx, session.ID, schema.SessionTyping)
		assert.ErrorIs(t, err, llm.ErrServiceUnavailable)
	})

	t.Run("Elsewhere", func(t *testing.T) {
		// The changes which others make to the store's sessions, which it
		// sends, are published, while the registry runs
		r, store := newRegistry(t)
		ch := watch(t, r)
		runCtx, stop := context.WithCancel(ctx)
		running := make(chan error, 1)
		go func() { running <- r.Run(runCtx, nil) }()
		event := sessionEvent(t, eventschema.EventUpdate, uuid.New(), serial(7))
		require.Eventually(t, func() bool { return store.Changed(event) > 0 }, 5*time.Second, time.Millisecond)
		assert.Equal(t, event, nextEvent(t, ch))
		stop()
		require.NoError(t, <-running)
	})

	t.Run("DeletedElsewhere", func(t *testing.T) {
		// A session which is deleted elsewhere is forgotten, so it's not
		// published as idle after it
		timeout := registry.TypingTimeout
		registry.TypingTimeout = 20 * time.Millisecond
		t.Cleanup(func() { registry.TypingTimeout = timeout })
		r, store := newRegistry(t)
		session, err := r.CreateSession(ctx, meta)
		require.NoError(t, err)
		_, err = r.SetSessionStatus(ctx, session.ID, schema.SessionTyping)
		require.NoError(t, err)
		ch := watch(t, r)
		runCtx, stop := context.WithCancel(ctx)
		running := make(chan error, 1)
		go func() { running <- r.Run(runCtx, nil) }()
		deleted := sessionEvent(t, eventschema.EventDelete, session.ID, nil)
		require.Eventually(t, func() bool { return store.Changed(deleted) > 0 }, 5*time.Second, time.Millisecond)
		assert.Equal(t, deleted, nextEvent(t, ch))
		time.Sleep(60 * time.Millisecond)
		assert.Empty(t, events(ch))
		stop()
		require.NoError(t, <-running)
	})
}

func Test_SessionThinking(t *testing.T) {
	// A reply is thinking as its model thinks, and as its tools run, and
	// replying as it writes text, and "Thinking" is logged to its stream,
	// once, the first time it's thinking
	ctx := context.Background()
	type streamed struct {
		what   llm.StreamType
		text   string
		status schema.SessionStatus
	}
	thinking := func(thought string, reply *providerschema.Message) *providerschema.Message {
		reply.Content = append([]providerschema.ContentBlock{{Thinking: types.Ptr(thought)}}, reply.Content...)
		return reply
	}
	withText := func(text string, reply *providerschema.Message) *providerschema.Message {
		reply.Content = append([]providerschema.ContentBlock{{Text: types.Ptr(text)}}, reply.Content...)
		return reply
	}
	run := func(t *testing.T, replies ...*providerschema.Message) ([]streamed, []schema.SessionStatus) {
		t.Helper()
		tk, err := toolkit.New(ctx)
		require.NoError(t, err)
		require.NoError(t, tk.Add(toolkit.NewNamespace("weather", "", namedEcho{name: "current"})))
		r, err := registry.New(registry.WithSessionStore(test.NewSessionStore()), registry.WithToolkit(tk))
		require.NoError(t, err)
		require.NoError(t, r.Add("", newScripted(replies...)))
		session, err := r.CreateSession(ctx, schema.SessionMeta{GeneratorMeta: schema.GeneratorMeta{Model: types.Ptr("model"), Tools: []string{"weather:*"}}})
		require.NoError(t, err)
		ch := watch(t, r)
		var got []streamed
		_, err = r.Chat(ctx, session.ID, []llm.Message{user(t, "Weather?")}, func(what llm.StreamType, text string) {
			current, err := r.GetSession(ctx, session.ID)
			require.NoError(t, err)
			got = append(got, streamed{what, text, current.Status})
			if current.Status == schema.SessionThinking {
				// It can't be deleted as it's thinking, as it can't as it's
				// replying
				_, err := r.DeleteSession(ctx, session.ID)
				assert.ErrorIs(t, err, llm.ErrConflict)
			}
		})
		require.NoError(t, err)
		var statuses []schema.SessionStatus
		for _, event := range events(ch) {
			var data schema.SessionEvent
			if event.Table == registry.SessionTable && json.Unmarshal(event.Data, &data) == nil && data.Status != "" {
				statuses = append(statuses, data.Status)
			}
		}
		current, err := r.GetSession(ctx, session.ID)
		require.NoError(t, err)
		assert.Equal(t, schema.SessionIdle, current.Status)

		// A client can't set it thinking
		_, err = r.SetSessionStatus(ctx, session.ID, schema.SessionThinking)
		assert.ErrorIs(t, err, llm.ErrBadParameter)
		return got, statuses
	}

	t.Run("ThinkToolThinkReply", func(t *testing.T) {
		// Thinking, as it thinks, calls a tool, and thinks again, until it
		// writes its reply
		got, statuses := run(t, thinking("Hmm", call("weather:current", `{"city":"Berlin"}`)), thinking("Again", text("Sunny")))
		assert.Equal(t, []streamed{
			{llm.StreamLog, registry.ThinkingLog, schema.SessionThinking},
			{llm.StreamThinking, "Hmm", schema.SessionThinking},
			{llm.StreamThinking, "\n\nAgain", schema.SessionThinking}, // after a blank line, as it's another turn
			{llm.StreamText, "Sunny", schema.SessionReplying},
		}, got)
		assert.Equal(t, []schema.SessionStatus{schema.SessionReplying, schema.SessionThinking, schema.SessionReplying, schema.SessionIdle}, statuses)
	})

	t.Run("TextBeforeTools", func(t *testing.T) {
		// Text before it calls a tool is replying, and it's thinking again as
		// the tool runs, but "Thinking" is logged once
		got, statuses := run(t, thinking("Hmm", withText("Let me look.", call("weather:current", `{"city":"Berlin"}`))), thinking("Again", text("Sunny")))
		var logs int
		for _, s := range got {
			if s.what == llm.StreamLog && s.text == registry.ThinkingLog {
				logs++
			}
		}
		assert.Equal(t, 1, logs)
		assert.Equal(t, []schema.SessionStatus{
			schema.SessionReplying, schema.SessionThinking, schema.SessionReplying, schema.SessionThinking, schema.SessionReplying, schema.SessionIdle,
		}, statuses)
		assert.Equal(t, schema.SessionThinking, got[len(got)-2].status, "it's thinking, as it thinks again")
		assert.Equal(t, streamed{llm.StreamText, "Sunny", schema.SessionReplying}, got[len(got)-1])
	})

	t.Run("ToolsWithoutThinking", func(t *testing.T) {
		// A model which calls tools without thinking is thinking as they
		// run, which is logged, until it replies
		got, statuses := run(t, call("weather:current", `{"city":"Berlin"}`), text("Sunny"))
		assert.Equal(t, []streamed{
			{llm.StreamLog, registry.ThinkingLog, schema.SessionThinking},
			{llm.StreamText, "Sunny", schema.SessionReplying},
		}, got)
		assert.Equal(t, []schema.SessionStatus{schema.SessionReplying, schema.SessionThinking, schema.SessionReplying, schema.SessionIdle}, statuses)
	})
}

func Test_SessionTrace(t *testing.T) {
	// Whether a session is traced is set as it's created, changed as it's
	// updated, and kept, as its other settings are
	ctx := context.Background()
	r, err := registry.New(registry.WithSessionStore(test.NewSessionStore()))
	require.NoError(t, err)
	require.NoError(t, r.Add("", newScripted()))
	session, err := r.CreateSession(ctx, schema.SessionMeta{GeneratorMeta: schema.GeneratorMeta{Model: types.Ptr("model")}})
	require.NoError(t, err)
	assert.Nil(t, session.Trace)
	assert.False(t, session.Traced(), "it isn't traced, unless it's set")

	traced, err := r.CreateSession(ctx, schema.SessionMeta{Trace: types.Ptr(true), GeneratorMeta: schema.GeneratorMeta{Model: types.Ptr("model")}})
	require.NoError(t, err)
	assert.True(t, traced.Traced())

	updated, err := r.UpdateSession(ctx, traced.ID, schema.SessionMeta{Title: types.Ptr("Weather")})
	require.NoError(t, err)
	assert.True(t, updated.Traced(), "it's kept, as another setting changes")
	updated, err = r.UpdateSession(ctx, traced.ID, schema.SessionMeta{Trace: types.Ptr(false)})
	require.NoError(t, err)
	assert.False(t, updated.Traced())
	require.NotNil(t, updated.Trace, "it's set off, rather than unset")
	got, err := r.GetSession(ctx, traced.ID)
	require.NoError(t, err)
	assert.False(t, got.Traced(), "it's stored")
}
