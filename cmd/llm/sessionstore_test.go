//go:build !client

package main

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
	types "github.com/mutablelogic/go-server/pkg/types"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

func Test_SessionStore(t *testing.T) {
	ctx := context.Background()
	meta := schema.SessionMeta{GeneratorMeta: schema.GeneratorMeta{
		Model:  types.Ptr("ollama:llama3.2"),
		System: types.Ptr("Be brief"),
		Tools:  []string{"memory:*"},
		Opt:    map[string]json.RawMessage{"temperature": json.RawMessage(`0.5`)},
	}}

	t.Run("Empty", func(t *testing.T) {
		store, err := newSessionStore(&memDefaults{})
		require.NoError(t, err)
		list, err := store.List(ctx, schema.SessionListRequest{})
		require.NoError(t, err)
		assert.Zero(t, list.Count)
		assert.Empty(t, list.Body)
		_, err = store.Get(ctx, uuid.New())
		assert.ErrorIs(t, err, llm.ErrNotFound)
	})

	t.Run("Kept", func(t *testing.T) {
		// A session, with its settings, messages and usage, is kept when the
		// defaults are read again, as when the server restarts
		d := &memDefaults{}
		store, err := newSessionStore(d)
		require.NoError(t, err)
		created, err := store.Create(ctx, meta)
		require.NoError(t, err)
		assert.NotEqual(t, uuid.Nil(), created.ID)
		assert.Equal(t, uint64(1), created.Serial)
		assert.False(t, created.Created.IsZero())

		created.Messages = []providerschema.Message{
			{MessageMeta: providerschema.MessageMeta{Sender: "user", Content: []providerschema.ContentBlock{{Text: types.Ptr("Hi")}}}, Tokens: 4},
			{MessageMeta: providerschema.MessageMeta{Sender: "assistant", Content: []providerschema.ContentBlock{{Text: types.Ptr("Hello")}}}, Tokens: 5, Result: providerschema.ResultStop},
		}
		created.Usage = schema.SessionUsage{Usage: llm.Usage{InputTokens: 10, OutputTokens: 5}, Input: 4, Output: 5, Overhead: 6}
		put, err := store.Put(ctx, created)
		require.NoError(t, err)
		assert.Equal(t, uint64(2), put.Serial)
		assert.Equal(t, created.Created, put.Created)

		d.Reload(t)
		store, err = newSessionStore(d)
		require.NoError(t, err)
		got, err := store.Get(ctx, created.ID)
		require.NoError(t, err)
		assert.Equal(t, put.ID, got.ID)
		assert.Equal(t, put.Serial, got.Serial)
		assert.True(t, put.Created.Equal(got.Created))
		assert.True(t, put.Modified.Equal(got.Modified))
		assert.Equal(t, meta.Model, got.Model)
		assert.Equal(t, meta.System, got.System)
		assert.Equal(t, meta.Tools, got.Tools)
		assert.JSONEq(t, `0.5`, string(got.Opt["temperature"]))
		require.Len(t, got.Messages, 2)
		assert.Equal(t, "Hello", got.Messages[1].Text())
		assert.Equal(t, uint64(5), got.Messages[1].Tokens)
		assert.Equal(t, providerschema.ResultStop, got.Messages[1].Result)
		assert.Equal(t, put.Usage, got.Usage)
	})

	t.Run("List", func(t *testing.T) {
		// Sessions are listed without their messages, most recently modified
		// first, from the offset, and at most the limit of them
		store, err := newSessionStore(&memDefaults{})
		require.NoError(t, err)
		var ids []uuid.UUID
		for range 3 {
			session, err := store.Create(ctx, meta)
			require.NoError(t, err)
			ids = append(ids, session.ID)
			time.Sleep(time.Millisecond)
		}
		first, err := store.Get(ctx, ids[0])
		require.NoError(t, err)
		first.Messages = []providerschema.Message{{MessageMeta: providerschema.MessageMeta{Sender: "user", Content: []providerschema.ContentBlock{{Text: types.Ptr("Hi")}}}}}
		_, err = store.Put(ctx, first)
		require.NoError(t, err)

		list, err := store.List(ctx, schema.SessionListRequest{})
		require.NoError(t, err)
		assert.Equal(t, uint(3), list.Count)
		var listed []uuid.UUID
		for _, session := range list.Body {
			listed = append(listed, session.ID)
			assert.Empty(t, session.Messages)
		}
		assert.Equal(t, []uuid.UUID{ids[0], ids[2], ids[1]}, listed)

		page, err := store.List(ctx, schema.SessionListRequest{Offset: 1, Limit: types.Ptr(uint(1))})
		require.NoError(t, err)
		assert.Equal(t, uint(3), page.Count)
		require.Len(t, page.Body, 1)
		assert.Equal(t, ids[2], page.Body[0].ID)

		// Listing doesn't change the sessions' messages
		got, err := store.Get(ctx, ids[0])
		require.NoError(t, err)
		assert.Len(t, got.Messages, 1)
	})

	t.Run("Conflict", func(t *testing.T) {
		// A session is changed, or deleted, only as it was last seen
		store, err := newSessionStore(&memDefaults{})
		require.NoError(t, err)
		created, err := store.Create(ctx, meta)
		require.NoError(t, err)
		_, err = store.Put(ctx, created)
		require.NoError(t, err)
		_, err = store.Put(ctx, created)
		assert.ErrorIs(t, err, llm.ErrConflict)
		assert.ErrorIs(t, store.Delete(ctx, created.ID, created.Serial), llm.ErrConflict)
		_, err = store.Put(ctx, schema.Session{ID: uuid.New()})
		assert.ErrorIs(t, err, llm.ErrNotFound)
	})

	t.Run("Delete", func(t *testing.T) {
		// Deleting the last session removes the key from the defaults, and a
		// session which isn't there is deleted already
		d := &memDefaults{}
		store, err := newSessionStore(d)
		require.NoError(t, err)
		created, err := store.Create(ctx, meta)
		require.NoError(t, err)
		require.NoError(t, store.Delete(ctx, created.ID, created.Serial))
		_, err = store.Get(ctx, created.ID)
		assert.ErrorIs(t, err, llm.ErrNotFound)
		assert.Nil(t, d.Get(sessionsKey))
		assert.NoError(t, store.Delete(ctx, created.ID, created.Serial))
	})

	t.Run("Registry", func(t *testing.T) {
		// The registry creates, changes and deletes sessions in the store
		store, err := newSessionStore(&memDefaults{})
		require.NoError(t, err)
		r, err := registry.New(registry.WithSessionStore(store))
		require.NoError(t, err)
		meta := schema.SessionMeta{GeneratorMeta: schema.GeneratorMeta{System: meta.System, Opt: meta.Opt}}
		created, err := r.CreateSession(ctx, meta)
		require.NoError(t, err)
		updated, err := r.UpdateSession(ctx, created.ID, schema.SessionMeta{GeneratorMeta: schema.GeneratorMeta{Opt: map[string]json.RawMessage{"temperature": json.RawMessage(`null`)}}})
		require.NoError(t, err)
		assert.Empty(t, updated.Opt)
		assert.Equal(t, meta.System, updated.System)
		_, err = r.DeleteSession(ctx, created.ID)
		require.NoError(t, err)
		_, err = r.GetSession(ctx, created.ID)
		assert.ErrorIs(t, err, llm.ErrNotFound)
	})

	t.Run("NoDefaults", func(t *testing.T) {
		_, err := newSessionStore(nil)
		assert.ErrorIs(t, err, llm.ErrBadParameter)
	})
}
