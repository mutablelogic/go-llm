package registry_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"
	"uuid"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	providerschema "github.com/mutablelogic/go-llm/provider/schema"
	registry "github.com/mutablelogic/go-llm/registry"
	schema "github.com/mutablelogic/go-llm/registry/schema"
	test "github.com/mutablelogic/go-llm/registry/test"
	types "github.com/mutablelogic/go-server/pkg/types"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

///////////////////////////////////////////////////////////////////////////////
// TESTS

func Test_Sessions(t *testing.T) {
	ctx := context.Background()
	meta := schema.SessionMeta{GeneratorMeta: schema.GeneratorMeta{Model: types.Ptr("model"), System: types.Ptr("Be brief")}}
	newRegistry := func(t *testing.T) (*registry.Registry, *test.SessionStore) {
		t.Helper()
		store := test.NewSessionStore()
		r, err := registry.New(registry.WithSessionStore(store), registry.WithToolkit(test.NewToolkit(t)))
		require.NoError(t, err)
		p := newScripted()
		p.models = []string{"model", "other"}
		require.NoError(t, r.Add("", p))
		return r, store
	}

	t.Run("Title", func(t *testing.T) {
		// A session is created with its title, without the space around it,
		// or none, when it's empty, and it's changed, or cleared, when it's
		// updated, and kept, when it isn't
		r, _ := newRegistry(t)
		session, err := r.CreateSession(ctx, schema.SessionMeta{Title: types.Ptr("  Trip to Lisbon "), GeneratorMeta: meta.GeneratorMeta})
		require.NoError(t, err)
		assert.Equal(t, "Trip to Lisbon", types.Value(session.Title))
		untitled, err := r.CreateSession(ctx, schema.SessionMeta{Title: types.Ptr(" "), GeneratorMeta: meta.GeneratorMeta})
		require.NoError(t, err)
		assert.Nil(t, untitled.Title)

		updated, err := r.UpdateSession(ctx, session.ID, schema.SessionMeta{GeneratorMeta: schema.GeneratorMeta{System: types.Ptr("Be kind")}})
		require.NoError(t, err)
		assert.Equal(t, "Trip to Lisbon", types.Value(updated.Title))
		updated, err = r.UpdateSession(ctx, session.ID, schema.SessionMeta{Title: types.Ptr("Lisbon")})
		require.NoError(t, err)
		assert.Equal(t, "Lisbon", types.Value(updated.Title))
		assert.Equal(t, "Be kind", types.Value(updated.System))
		updated, err = r.UpdateSession(ctx, session.ID, schema.SessionMeta{Title: types.Ptr("")})
		require.NoError(t, err)
		assert.Nil(t, updated.Title)
	})

	t.Run("Create", func(t *testing.T) {
		// A session is created with the settings, and no messages
		r, _ := newRegistry(t)
		session, err := r.CreateSession(ctx, meta)
		require.NoError(t, err)
		assert.NotEqual(t, uuid.Nil(), session.ID)
		assert.Equal(t, uint64(1), session.Serial)
		assert.False(t, session.Created.IsZero())
		assert.Equal(t, "scripted:model", types.Value(session.Model))
		assert.Equal(t, meta.System, session.System)
		assert.Empty(t, session.Messages)
	})

	t.Run("CreateInvalid", func(t *testing.T) {
		// The settings must have a JSON schema, options which aren't fields,
		// and a model which exists, but needn't have a model, as it can be
		// inherited
		r, store := newRegistry(t)
		for name, meta := range map[string]schema.SessionMeta{
			"Schema": {GeneratorMeta: schema.GeneratorMeta{Model: types.Ptr("model"), Schema: json.RawMessage(`{"type":1}`)}},
			"Option": {GeneratorMeta: schema.GeneratorMeta{Model: types.Ptr("model"), Opt: map[string]json.RawMessage{llm.SystemPromptKey: json.RawMessage(`"x"`)}}},
		} {
			_, err := r.CreateSession(ctx, meta)
			assert.ErrorIs(t, err, llm.ErrBadParameter, name)
		}
		_, err := r.CreateSession(ctx, schema.SessionMeta{GeneratorMeta: schema.GeneratorMeta{Model: types.Ptr("missing")}})
		assert.ErrorIs(t, err, llm.ErrNotFound)
		assert.Zero(t, store.Len())
		session, err := r.CreateSession(ctx, schema.SessionMeta{})
		require.NoError(t, err)
		assert.Nil(t, session.Model)
	})

	t.Run("Tools", func(t *testing.T) {
		// The tools are the toolkit's, by their canonical names, and a
		// namespace and "*", or "*", are kept as they are
		r, _ := newRegistry(t)
		session, err := r.CreateSession(ctx, schema.SessionMeta{GeneratorMeta: schema.GeneratorMeta{Tools: []string{"current", "memory:*"}}})
		require.NoError(t, err)
		assert.Equal(t, []string{"weather:current", "memory:*"}, session.Tools)
		all, err := r.CreateSession(ctx, schema.SessionMeta{GeneratorMeta: schema.GeneratorMeta{Tools: []string{"*"}}})
		require.NoError(t, err)
		assert.Equal(t, []string{"*"}, all.Tools)

		session, err = r.UpdateSession(ctx, session.ID, schema.SessionMeta{GeneratorMeta: schema.GeneratorMeta{Tools: []string{"forecast"}}})
		require.NoError(t, err)
		assert.Equal(t, []string{"weather:forecast"}, session.Tools)

		// A tool which isn't the toolkit's isn't valid, and isn't stored
		_, err = r.CreateSession(ctx, schema.SessionMeta{GeneratorMeta: schema.GeneratorMeta{Tools: []string{"null"}}})
		assert.ErrorIs(t, err, llm.ErrNotFound)
		_, err = r.UpdateSession(ctx, session.ID, schema.SessionMeta{GeneratorMeta: schema.GeneratorMeta{Tools: []string{"missing:*"}}})
		assert.ErrorIs(t, err, llm.ErrNotFound)
		got, err := r.GetSession(ctx, session.ID)
		require.NoError(t, err)
		assert.Equal(t, []string{"weather:forecast"}, got.Tools)

		// An empty list clears them, so there are none
		session, err = r.UpdateSession(ctx, session.ID, schema.SessionMeta{GeneratorMeta: schema.GeneratorMeta{Tools: []string{}}})
		require.NoError(t, err)
		assert.Nil(t, session.Tools)

		// Without a toolkit, there are no tools
		noTools, err := registry.New(registry.WithSessionStore(test.NewSessionStore()))
		require.NoError(t, err)
		_, err = noTools.CreateSession(ctx, schema.SessionMeta{GeneratorMeta: schema.GeneratorMeta{Tools: []string{"*"}}})
		assert.ErrorIs(t, err, llm.ErrNotFound)
		_, err = noTools.CreateSession(ctx, schema.SessionMeta{})
		assert.NoError(t, err)
	})

	t.Run("Get", func(t *testing.T) {
		r, _ := newRegistry(t)
		created, err := r.CreateSession(ctx, meta)
		require.NoError(t, err)
		session, err := r.GetSession(ctx, created.ID)
		require.NoError(t, err)
		assert.Equal(t, created, session)
		_, err = r.GetSession(ctx, uuid.New())
		assert.ErrorIs(t, err, llm.ErrNotFound)
	})

	t.Run("List", func(t *testing.T) {
		// Sessions are listed without their messages, most recently modified
		// first, from the offset, and at most the limit of them
		r, store := newRegistry(t)
		var created []uuid.UUID
		for range 3 {
			session, err := r.CreateSession(ctx, meta)
			require.NoError(t, err)
			created = append(created, session.ID)
			time.Sleep(time.Millisecond)
		}
		first, err := store.Get(ctx, created[0])
		require.NoError(t, err)
		first.Messages = []providerschema.Message{{MessageMeta: providerschema.MessageMeta{Sender: "user", Content: []providerschema.ContentBlock{{Text: types.Ptr("Hi")}}}}}
		_, err = store.Put(ctx, first)
		require.NoError(t, err)

		list, err := r.ListSessions(ctx, schema.SessionListRequest{})
		require.NoError(t, err)
		assert.Equal(t, uint(3), list.Count)
		var ids []uuid.UUID
		for _, session := range list.Body {
			ids = append(ids, session.ID)
			assert.Empty(t, session.Messages)
		}
		assert.Equal(t, []uuid.UUID{created[0], created[2], created[1]}, ids)

		list, err = r.ListSessions(ctx, schema.SessionListRequest{Offset: 1, Limit: types.Ptr(uint(1))})
		require.NoError(t, err)
		assert.Equal(t, uint(3), list.Count)
		require.Len(t, list.Body, 1)
		assert.Equal(t, created[2], list.Body[0].ID)
	})

	t.Run("Update", func(t *testing.T) {
		// What the update sets is changed, the rest is kept, as are the
		// messages
		r, store := newRegistry(t)
		created, err := r.CreateSession(ctx, meta)
		require.NoError(t, err)
		stored, err := store.Get(ctx, created.ID)
		require.NoError(t, err)
		stored.Messages = []providerschema.Message{{MessageMeta: providerschema.MessageMeta{Sender: "user", Content: []providerschema.ContentBlock{{Text: types.Ptr("Hi")}}}}}
		_, err = store.Put(ctx, stored)
		require.NoError(t, err)

		other := schema.SessionMeta{GeneratorMeta: schema.GeneratorMeta{Model: types.Ptr("other"), Tools: []string{"memory:*"}}}
		session, err := r.UpdateSession(ctx, created.ID, other)
		require.NoError(t, err)
		assert.Equal(t, schema.SessionMeta{GeneratorMeta: schema.GeneratorMeta{Model: types.Ptr("scripted:other"), System: types.Ptr("Be brief"), Tools: []string{"memory:*"}}}, session.SessionMeta)
		assert.Empty(t, session.Messages)
		assert.Equal(t, uint64(3), session.Serial)
		messages, err := r.ListSessionMessages(ctx, created.ID, schema.MessageListRequest{})
		require.NoError(t, err)
		assert.Equal(t, uint(1), messages.Count)

		// An update which sets nothing isn't valid
		_, err = r.UpdateSession(ctx, created.ID, schema.SessionMeta{})
		assert.ErrorIs(t, err, llm.ErrBadParameter)

		// A model which isn't the registry's isn't valid, and an empty one
		// clears it, without being checked
		_, err = r.UpdateSession(ctx, created.ID, schema.SessionMeta{GeneratorMeta: schema.GeneratorMeta{Model: types.Ptr("missing")}})
		assert.ErrorIs(t, err, llm.ErrNotFound)
		session, err = r.UpdateSession(ctx, created.ID, schema.SessionMeta{GeneratorMeta: schema.GeneratorMeta{Model: types.Ptr("")}})
		require.NoError(t, err)
		assert.Nil(t, session.Model)

		// The settings, as changed, must be valid
		_, err = r.UpdateSession(ctx, uuid.New(), other)
		assert.ErrorIs(t, err, llm.ErrNotFound)
		_, err = r.UpdateSession(ctx, created.ID, schema.SessionMeta{GeneratorMeta: schema.GeneratorMeta{Opt: map[string]json.RawMessage{llm.SystemPromptKey: json.RawMessage(`"x"`)}}})
		assert.ErrorIs(t, err, llm.ErrBadParameter)
		stored, err = store.Get(ctx, created.ID)
		require.NoError(t, err)
		assert.Empty(t, stored.Opt)
	})

	t.Run("Delete", func(t *testing.T) {
		// The session is returned, as it was, and is gone
		r, _ := newRegistry(t)
		created, err := r.CreateSession(ctx, meta)
		require.NoError(t, err)
		deleted, err := r.DeleteSession(ctx, created.ID)
		require.NoError(t, err)
		assert.Equal(t, created, deleted)
		_, err = r.GetSession(ctx, created.ID)
		assert.ErrorIs(t, err, llm.ErrNotFound)
		_, err = r.DeleteSession(ctx, created.ID)
		assert.ErrorIs(t, err, llm.ErrNotFound)
	})

	t.Run("Snapshot", func(t *testing.T) {
		// A session which is returned doesn't share what's stored
		r, _ := newRegistry(t)
		created, err := r.CreateSession(ctx, meta)
		require.NoError(t, err)
		*created.System = "Changed"
		got, err := r.GetSession(ctx, created.ID)
		require.NoError(t, err)
		assert.Equal(t, "Be brief", types.Value(got.System))
	})

	t.Run("NoStore", func(t *testing.T) {
		// Without a session store, sessions aren't available
		r, err := registry.New()
		require.NoError(t, err)
		_, err = r.CreateSession(ctx, meta)
		assert.ErrorIs(t, err, llm.ErrServiceUnavailable)
		_, err = r.GetSession(ctx, uuid.New())
		assert.ErrorIs(t, err, llm.ErrServiceUnavailable)
		_, err = r.ListSessions(ctx, schema.SessionListRequest{})
		assert.ErrorIs(t, err, llm.ErrServiceUnavailable)
		_, err = r.UpdateSession(ctx, uuid.New(), meta)
		assert.ErrorIs(t, err, llm.ErrServiceUnavailable)
		_, err = r.DeleteSession(ctx, uuid.New())
		assert.ErrorIs(t, err, llm.ErrServiceUnavailable)

		_, err = registry.New(registry.WithSessionStore(nil))
		assert.ErrorIs(t, err, llm.ErrBadParameter)
	})
}

func Test_Chat(t *testing.T) {
	ctx := context.Background()
	hi := func(text string) []llm.Message { return []llm.Message{user(t, text)} }
	newChat := func(t *testing.T, replies ...*providerschema.Message) (*registry.Registry, *test.SessionStore, *scripted) {
		t.Helper()
		store := test.NewSessionStore()
		r, err := registry.New(registry.WithSessionStore(store), registry.WithToolkit(test.NewToolkit(t)))
		require.NoError(t, err)
		p := newScripted(replies...)
		require.NoError(t, r.Add("", p))
		return r, store, p
	}
	meta := schema.SessionMeta{GeneratorMeta: schema.GeneratorMeta{Model: types.Ptr("model"), System: types.Ptr("Be brief")}}

	t.Run("Turns", func(t *testing.T) {
		// Each turn's messages, and the reply, are added to the session, with
		// the usage, and the next turn sends them
		r, store, p := newChat(t, text("Hello"), text("Still here"))
		session, err := r.CreateSession(ctx, meta)
		require.NoError(t, err)

		response, err := r.Chat(ctx, session.ID, hi("Hi"), nil)
		require.NoError(t, err)
		assert.Equal(t, session.ID, response.ID)
		assert.Equal(t, uint64(2), response.Serial)
		require.Len(t, response.Messages, 2)
		assert.Equal(t, "Hello", response.Messages[1].Text())

		response, err = r.Chat(ctx, session.ID, hi("Are you?"), nil)
		require.NoError(t, err)
		assert.Equal(t, uint64(3), response.Serial)
		require.Len(t, p.asked, 2)
		require.Len(t, p.asked[1].messages, 3)
		assert.Equal(t, "Hi", p.asked[1].messages[0].Text())
		assert.Equal(t, "Hello", p.asked[1].messages[1].Text())
		system, _ := llm.GetOpt[string](p.asked[1].opts, llm.SystemPromptKey)
		assert.Equal(t, "Be brief", system)

		// The session has every message, each with its tokens, and the usage
		// of both turns, and the tokens of its conversation, as of the last
		stored, err := store.Get(ctx, session.ID)
		require.NoError(t, err)
		require.Len(t, stored.Messages, 4)
		for _, message := range stored.Messages {
			assert.NotZero(t, message.Tokens)
		}
		assert.Equal(t, "Still here", stored.Messages[3].Text())
		assert.Equal(t, llm.Usage{InputTokens: 20, OutputTokens: 10}, withoutDuration(stored.Usage.Usage))
		assert.Equal(t, response.Usage.Input, stored.Usage.Input)
		assert.Equal(t, uint(10), stored.Usage.Output)
	})

	t.Run("Replies", func(t *testing.T) {
		// Each reply which is stored is given to the function, with its
		// session, and a reply which isn't stored isn't
		type reply struct {
			session uuid.UUID
			text    string
		}
		var got []reply
		store := test.NewSessionStore()
		r, err := registry.New(registry.WithSessionStore(store), registry.WithToolkit(test.NewToolkit(t)), registry.WithReplies(func(ctx context.Context, session uuid.UUID, text string) {
			assert.NoError(t, ctx.Err())
			got = append(got, reply{session, text})
		}))
		require.NoError(t, err)
		require.NoError(t, r.Add("", newScripted(text("Hello"), text("Again"))))
		session, err := r.CreateSession(ctx, meta)
		require.NoError(t, err)

		// The context the reply is given with isn't cancelled with the
		// chat's
		chatCtx, cancel := context.WithCancel(ctx)
		_, err = r.Chat(chatCtx, session.ID, hi("Hi"), nil)
		cancel()
		require.NoError(t, err)
		_, err = r.Chat(ctx, session.ID, hi("Hi again"), nil)
		require.NoError(t, err)
		assert.Equal(t, []reply{{session.ID, "Hello"}, {session.ID, "Again"}}, got)

		_, err = r.Chat(ctx, uuid.New(), hi("Nobody"), nil)
		assert.ErrorIs(t, err, llm.ErrNotFound)
		assert.Len(t, got, 2)
	})

	t.Run("Stream", func(t *testing.T) {
		r, _, _ := newChat(t, text("Hello"))
		session, err := r.CreateSession(ctx, meta)
		require.NoError(t, err)
		var streamed string
		_, err = r.Chat(ctx, session.ID, hi("Hi"), func(_ llm.StreamType, text string) { streamed += text })
		require.NoError(t, err)
		assert.Equal(t, "Hello", streamed)
	})

	t.Run("Conflict", func(t *testing.T) {
		// The session is changed as the model replies, so the reply isn't
		// stored
		r, store, _ := newChat(t, text("Hello"))
		session, err := r.CreateSession(ctx, meta)
		require.NoError(t, err)
		_, err = r.Chat(ctx, session.ID, hi("Hi"), func(llm.StreamType, string) {
			_, err := r.UpdateSession(ctx, session.ID, schema.SessionMeta{GeneratorMeta: schema.GeneratorMeta{System: types.Ptr("Be thorough")}})
			require.NoError(t, err)
		})
		assert.ErrorIs(t, err, llm.ErrConflict)
		stored, err := store.Get(ctx, session.ID)
		require.NoError(t, err)
		assert.Empty(t, stored.Messages)
		assert.Equal(t, "Be thorough", types.Value(stored.System))
	})

	t.Run("NoModel", func(t *testing.T) {
		// A session without a model can't be chatted with
		r, store, p := newChat(t, text("Never"))
		session, err := r.CreateSession(ctx, schema.SessionMeta{})
		require.NoError(t, err)
		_, err = r.Chat(ctx, session.ID, hi("Hi"), nil)
		assert.ErrorIs(t, err, llm.ErrBadParameter)
		assert.Empty(t, p.asked)
		stored, err := store.Get(ctx, session.ID)
		require.NoError(t, err)
		assert.Empty(t, stored.Messages)
	})

	t.Run("Errors", func(t *testing.T) {
		r, _, _ := newChat(t)
		_, err := r.Chat(ctx, uuid.New(), hi("Hi"), nil)
		assert.ErrorIs(t, err, llm.ErrNotFound)
		noStore, err := registry.New()
		require.NoError(t, err)
		_, err = noStore.Chat(ctx, uuid.New(), hi("Hi"), nil)
		assert.ErrorIs(t, err, llm.ErrServiceUnavailable)
	})
}
