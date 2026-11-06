//go:build !client

package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
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
	newMeta := func() schema.SessionMeta {
		return schema.SessionMeta{GeneratorMeta: schema.GeneratorMeta{
			Model:  types.Ptr("ollama:llama3.2"),
			System: types.Ptr("Be brief"),
			Tools:  []string{"memory:*"},
			Opt:    map[string]json.RawMessage{"temperature": json.RawMessage(`0.5`)},
		}}
	}
	newStore := func(t *testing.T, dir string) *sessionStore {
		t.Helper()
		store, err := newSessionStore(dir)
		require.NoError(t, err)
		return store
	}

	t.Run("Empty", func(t *testing.T) {
		store := newStore(t, filepath.Join(t.TempDir(), "llm"))
		list, err := store.List(ctx, schema.SessionListRequest{})
		require.NoError(t, err)
		assert.Zero(t, list.Count)
		_, err = store.Get(ctx, uuid.New())
		assert.ErrorIs(t, err, llm.ErrNotFound)
	})

	t.Run("Kept", func(t *testing.T) {
		// A session, with its settings, messages and usage, is kept in a file
		// of its own, which only the user can read, when the store is made
		// again, as when the server restarts
		dir := t.TempDir()
		store := newStore(t, dir)
		created, err := store.Create(ctx, uuid.Nil(), newMeta())
		require.NoError(t, err)
		assert.NotEqual(t, uuid.Nil(), created.ID)
		assert.Equal(t, uint64(1), created.Serial)

		created.Messages = []providerschema.Message{
			{MessageMeta: providerschema.MessageMeta{Sender: "user", Content: []providerschema.ContentBlock{{Text: types.Ptr("Hi")}}}, Tokens: 4},
			{MessageMeta: providerschema.MessageMeta{Sender: "assistant", Content: []providerschema.ContentBlock{{Text: types.Ptr("Hello")}}}, Tokens: 5, Result: providerschema.ResultStop},
		}
		created.Usage = schema.SessionUsage{Usage: llm.Usage{InputTokens: 10, OutputTokens: 5}, Input: 4, Output: 5, Overhead: 6}
		put, err := store.Put(ctx, created)
		require.NoError(t, err)
		assert.Equal(t, uint64(2), put.Serial)

		info, err := os.Stat(filepath.Join(dir, "session-"+created.ID.String()+".json"))
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
		entries, err := os.ReadDir(dir)
		require.NoError(t, err)
		assert.Len(t, entries, 1, "no temporary files are left")

		got, err := newStore(t, dir).Get(ctx, created.ID)
		require.NoError(t, err)
		assert.Equal(t, put.Serial, got.Serial)
		assert.True(t, put.Created.Equal(got.Created))
		assert.Equal(t, "Be brief", types.Value(got.System))
		require.Len(t, got.Messages, 2)
		assert.Equal(t, "Hello", got.Messages[1].Text())
		assert.Equal(t, uint64(5), got.Messages[1].Tokens)
		assert.Equal(t, providerschema.ResultStop, got.Messages[1].Result)
		assert.Equal(t, put.Usage, got.Usage)
	})

	t.Run("List", func(t *testing.T) {
		// Sessions are listed without their messages, most recently modified
		// first, from the offset, and at most the limit of them, and files
		// which aren't sessions are skipped
		dir := t.TempDir()
		store := newStore(t, dir)
		var ids []uuid.UUID
		for range 3 {
			session, err := store.Create(ctx, uuid.Nil(), newMeta())
			require.NoError(t, err)
			ids = append(ids, session.ID)
			time.Sleep(time.Millisecond)
		}
		first, err := store.Get(ctx, ids[0])
		require.NoError(t, err)
		first.Messages = []providerschema.Message{{MessageMeta: providerschema.MessageMeta{Sender: "user", Content: []providerschema.ContentBlock{{Text: types.Ptr("Hi")}}}}}
		_, err = store.Put(ctx, first)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(dir, "defaults.json"), []byte(`{}`), 0o600))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "session-not-a-uuid.json"), []byte(`{}`), 0o600))

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
		require.Len(t, page.Body, 1)
		assert.Equal(t, ids[2], page.Body[0].ID)
	})

	t.Run("Conflict", func(t *testing.T) {
		// A session is changed, or deleted, only as it was last seen
		store := newStore(t, t.TempDir())
		created, err := store.Create(ctx, uuid.Nil(), newMeta())
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
		// A session's file is removed, and a session which isn't there is
		// deleted already
		dir := t.TempDir()
		store := newStore(t, dir)
		created, err := store.Create(ctx, uuid.Nil(), newMeta())
		require.NoError(t, err)
		require.NoError(t, store.Delete(ctx, created.ID, created.Serial))
		_, err = store.Get(ctx, created.ID)
		assert.ErrorIs(t, err, llm.ErrNotFound)
		entries, err := os.ReadDir(dir)
		require.NoError(t, err)
		assert.Empty(t, entries)
		assert.NoError(t, store.Delete(ctx, created.ID, created.Serial))
	})

	t.Run("Snapshot", func(t *testing.T) {
		// A session which is returned doesn't share what's stored
		store := newStore(t, t.TempDir())
		created, err := store.Create(ctx, uuid.Nil(), newMeta())
		require.NoError(t, err)
		*created.System = "Changed"
		created.Opt["temperature"] = json.RawMessage(`1`)
		got, err := store.Get(ctx, created.ID)
		require.NoError(t, err)
		assert.Equal(t, "Be brief", types.Value(got.System))
		assert.JSONEq(t, `0.5`, string(got.Opt["temperature"]))
	})

	t.Run("Migrate", func(t *testing.T) {
		// Sessions which the defaults have are moved to files, apart from one
		// which has a file, and removed from the defaults
		dir := t.TempDir()
		store := newStore(t, dir)
		kept, err := store.Create(ctx, uuid.Nil(), newMeta())
		require.NoError(t, err)
		moved := schema.Session{ID: uuid.New(), Serial: 3, SessionMeta: newMeta(), Messages: []providerschema.Message{
			{MessageMeta: providerschema.MessageMeta{Sender: "user", Content: []providerschema.ContentBlock{{Text: types.Ptr("Hi")}}}},
		}}
		stale := kept
		stale.Serial = 9
		d := &memDefaults{}
		require.NoError(t, d.Set(sessionsKey, []schema.Session{moved, stale}))
		d.Reload(t)

		require.NoError(t, store.migrate(d))
		assert.Nil(t, d.Get(sessionsKey))
		got, err := store.Get(ctx, moved.ID)
		require.NoError(t, err)
		assert.Equal(t, uint64(3), got.Serial)
		assert.Len(t, got.Messages, 1)
		got, err = store.Get(ctx, kept.ID)
		require.NoError(t, err)
		assert.Equal(t, uint64(1), got.Serial)

		// Without sessions in the defaults, there's nothing to move
		assert.NoError(t, store.migrate(&memDefaults{}))
	})

	t.Run("Registry", func(t *testing.T) {
		// The registry creates, changes and deletes sessions in the store
		store := newStore(t, t.TempDir())
		r, err := registry.New(registry.WithSessionStore(store))
		require.NoError(t, err)
		meta := schema.SessionMeta{GeneratorMeta: schema.GeneratorMeta{System: types.Ptr("Be brief"), Opt: map[string]json.RawMessage{"temperature": json.RawMessage(`0.5`)}}}
		created, err := r.CreateSession(ctx, meta)
		require.NoError(t, err)
		updated, err := r.UpdateSession(ctx, created.ID, schema.SessionMeta{GeneratorMeta: schema.GeneratorMeta{Opt: map[string]json.RawMessage{"temperature": json.RawMessage(`null`)}}})
		require.NoError(t, err)
		assert.Empty(t, updated.Opt)
		_, err = r.DeleteSession(ctx, created.ID)
		require.NoError(t, err)
		_, err = r.GetSession(ctx, created.ID)
		assert.ErrorIs(t, err, llm.ErrNotFound)
	})

	t.Run("NoDirectory", func(t *testing.T) {
		_, err := newSessionStore(" ")
		assert.ErrorIs(t, err, llm.ErrBadParameter)
	})
}
