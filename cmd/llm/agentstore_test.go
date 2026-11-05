//go:build !client

package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	registry "github.com/mutablelogic/go-llm/registry"
	schema "github.com/mutablelogic/go-llm/registry/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

func Test_AgentStore(t *testing.T) {
	ctx := context.Background()
	newMeta := func(name string) schema.AgentMeta {
		return schema.AgentMeta{
			PromptMeta:    schema.PromptMeta{Name: name, Prompt: types.Ptr("Segment: {{ .transcript }}"), Input: json.RawMessage(`{"type":"object"}`)},
			GeneratorMeta: schema.GeneratorMeta{System: types.Ptr("Be brief"), Opt: map[string]json.RawMessage{"thinking": json.RawMessage(`false`)}},
		}
	}
	newStore := func(t *testing.T, dir string) *agentStore {
		t.Helper()
		store, err := newAgentStore(dir)
		require.NoError(t, err)
		return store
	}

	t.Run("Kept", func(t *testing.T) {
		// An agent is kept in a file of its own, which only the user can
		// read, when the store is made again, as when the server restarts
		dir := t.TempDir()
		store := newStore(t, dir)
		created, err := store.Create(ctx, newMeta("segment"))
		require.NoError(t, err)
		assert.NotZero(t, created.Serial)
		info, err := os.Stat(filepath.Join(dir, "agent-segment.json"))
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())

		got, err := newStore(t, dir).Get(ctx, "segment")
		require.NoError(t, err)
		assert.Equal(t, created.Serial, got.Serial)
		assert.Equal(t, "Segment: {{ .transcript }}", types.Value(got.Prompt))
		assert.JSONEq(t, `false`, string(got.Opt["thinking"]))
		_, err = store.Create(ctx, newMeta("segment"))
		assert.ErrorIs(t, err, llm.ErrConflict)
		_, err = store.Get(ctx, "missing")
		assert.ErrorIs(t, err, llm.ErrNotFound)
		_, err = store.Get(ctx, "../defaults")
		assert.ErrorIs(t, err, llm.ErrNotFound)
	})

	t.Run("List", func(t *testing.T) {
		// Agents are listed in order of their names, and files which aren't
		// agents are skipped
		dir := t.TempDir()
		store := newStore(t, dir)
		for _, name := range []string{"summarize", "segment", "translate"} {
			_, err := store.Create(ctx, newMeta(name))
			require.NoError(t, err)
		}
		require.NoError(t, os.WriteFile(filepath.Join(dir, "defaults.json"), []byte(`{}`), 0o600))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "agent-not a name.json"), []byte(`{}`), 0o600))
		list, err := store.List(ctx, schema.AgentListRequest{})
		require.NoError(t, err)
		assert.Equal(t, uint(3), list.Count)
		var names []string
		for _, agent := range list.Body {
			names = append(names, agent.Name)
		}
		assert.Equal(t, []string{"segment", "summarize", "translate"}, names)
		page, err := store.List(ctx, schema.AgentListRequest{Offset: 2, Limit: types.Ptr(uint(5))})
		require.NoError(t, err)
		require.Len(t, page.Body, 1)
		assert.Equal(t, "translate", page.Body[0].Name)
	})

	t.Run("Recreated", func(t *testing.T) {
		// An agent which is changed, or deleted, as it was last seen, and
		// one which is deleted, and created again, even by a store made
		// again, has a serial it didn't have
		dir := t.TempDir()
		store := newStore(t, dir)
		old, err := store.Create(ctx, newMeta("segment"))
		require.NoError(t, err)
		put, err := store.Put(ctx, old)
		require.NoError(t, err)
		assert.Greater(t, put.Serial, old.Serial)
		_, err = store.Put(ctx, old)
		assert.ErrorIs(t, err, llm.ErrConflict)
		assert.ErrorIs(t, store.Delete(ctx, "segment", old.Serial), llm.ErrConflict)
		require.NoError(t, store.Delete(ctx, "segment", put.Serial))
		assert.NoError(t, store.Delete(ctx, "segment", put.Serial))

		again := newStore(t, dir)
		created, err := again.Create(ctx, newMeta("segment"))
		require.NoError(t, err)
		assert.Greater(t, created.Serial, put.Serial)
		_, err = again.Put(ctx, put)
		assert.ErrorIs(t, err, llm.ErrConflict)
		_, err = again.Put(ctx, schema.Agent{AgentMeta: newMeta("missing")})
		assert.ErrorIs(t, err, llm.ErrNotFound)
	})

	t.Run("Registry", func(t *testing.T) {
		// The registry creates, changes and deletes agents in the store
		store := newStore(t, t.TempDir())
		r, err := registry.New(registry.WithAgentStore(store))
		require.NoError(t, err)
		_, err = r.CreateAgent(ctx, newMeta("segment"))
		require.NoError(t, err)
		updated, err := r.UpdateAgent(ctx, "segment", schema.AgentMeta{PromptMeta: schema.PromptMeta{Title: types.Ptr("Segment")}})
		require.NoError(t, err)
		assert.Equal(t, "Segment", types.Value(updated.Title))
		_, err = r.DeleteAgent(ctx, "segment")
		require.NoError(t, err)
		_, err = r.GetAgent(ctx, "segment")
		assert.ErrorIs(t, err, llm.ErrNotFound)
	})

	t.Run("NoDirectory", func(t *testing.T) {
		_, err := newAgentStore(" ")
		assert.ErrorIs(t, err, llm.ErrBadParameter)
	})
}
