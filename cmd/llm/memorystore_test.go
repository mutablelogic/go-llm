//go:build !client

package main

import (
	"os"
	"path/filepath"
	"testing"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	memory "github.com/mutablelogic/go-llm/memory"
	schema "github.com/mutablelogic/go-llm/memory/schema"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

func Test_MemoryStore(t *testing.T) {
	ctx := t.Context()
	path := filepath.Join(t.TempDir(), memoryFile)
	file, err := newJSONFile(path)
	require.NoError(t, err)
	store := newMemoryStore(file, filepath.Dir(path))

	// With no file, nothing is known
	details, err := store.Details(ctx)
	require.NoError(t, err)
	assert.Equal(t, schema.Details{}, details)

	// The details are kept in the file, beside what else is in it, and read
	// by a memory, as it starts
	require.NoError(t, file.Set("other", "value"))
	m, err := memory.New(ctx, store)
	require.NoError(t, err)
	_, err = m.SetDetail(ctx, schema.FieldName, &schema.Detail{Value: "David"})
	require.NoError(t, err)
	_, err = m.SetDetail(ctx, schema.FieldLocation, &schema.Detail{Declined: true})
	require.NoError(t, err)

	file, err = newJSONFile(path)
	require.NoError(t, err)
	m, err = memory.New(ctx, newMemoryStore(file, filepath.Dir(path)))
	require.NoError(t, err)
	assert.Equal(t, "David", m.Details().Get(schema.FieldName).Value)
	assert.True(t, m.Details().Get(schema.FieldLocation).Declined)
	assert.Equal(t, "value", file.Get("other"))

	// Details which aren't valid aren't read, or written
	require.NoError(t, os.WriteFile(path, []byte(`{"details":{"timezone":{"value":"Nowhere"}}}`), 0o600))
	_, err = store.Details(ctx)
	assert.ErrorIs(t, err, llm.ErrBadParameter)
	var details2 schema.Details
	details2.Timezone = &schema.Detail{Value: "Nowhere"}
	assert.ErrorIs(t, store.PutDetails(ctx, details2), llm.ErrBadParameter)
}

func Test_MemoryStoreMemories(t *testing.T) {
	ctx := t.Context()
	dir := t.TempDir()
	file, err := newJSONFile(filepath.Join(dir, memoryFile))
	require.NoError(t, err)
	store := newMemoryStore(file, dir)
	m, err := memory.New(ctx, store)
	require.NoError(t, err)

	// A memory is a file of Markdown, with its front matter
	_, _, err = m.Remember(ctx, schema.MemoryMeta{Key: "kitchen_lights", Description: "The kitchen's lights", Body: "- light.kitchen_main\n- light.kitchen_counter"}, "")
	require.NoError(t, err)
	data, err := os.ReadFile(filepath.Join(dir, "memory-kitchen_lights.md"))
	require.NoError(t, err)
	assert.Regexp(t, `^---\ndescription: The kitchen's lights\ncreated: .+\n---\n\n- light.kitchen_main\n- light.kitchen_counter\n$`, string(data))
	info, err := os.Stat(filepath.Join(dir, "memory-kitchen_lights.md"))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())

	// Read again, as it was written
	got, err := store.GetMemory(ctx, "kitchen_lights")
	require.NoError(t, err)
	assert.Equal(t, "- light.kitchen_main\n- light.kitchen_counter", got.Body)
	assert.False(t, got.Created.IsZero())
	assert.Empty(t, got.Version, "the store has no version, which the memory sets")

	// One edited by hand is read, and one which isn't valid, or isn't a
	// memory's, is left out of the list
	require.NoError(t, os.WriteFile(filepath.Join(dir, "memory-units.md"), []byte("---\r\ndescription: Units\r\n---\r\nMetric\r\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "memory-broken.md"), []byte("no front matter"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "memory-not a key.md"), []byte("---\ndescription: x\n---\nx\n"), 0o600))
	units, err := store.GetMemory(ctx, "units")
	require.NoError(t, err)
	assert.Equal(t, "Metric", units.Body)

	// Edited by hand, so its version, as the memory says, is of what it is
	before, err := m.GetMemory(ctx, "units")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "memory-units.md"), []byte("---\ndescription: Units\nversion: 7\n---\nImperial\n"), 0o600))
	after, err := m.GetMemory(ctx, "units")
	require.NoError(t, err)
	assert.NotEqual(t, before.Version, after.Version)
	_, _, err = m.Remember(ctx, schema.MemoryMeta{Key: "units", Description: "Units", Body: "Metric, again"}, before.Version)
	assert.ErrorIs(t, err, llm.ErrConflict, "a change made before the edit")
	_, err = store.GetMemory(ctx, "broken")
	assert.ErrorIs(t, err, llm.ErrInternalServerError)
	list, err := m.ListMemories(ctx, schema.MemoryListRequest{})
	require.NoError(t, err)
	assert.Equal(t, uint(2), list.Count)
	list, err = m.ListMemories(ctx, schema.MemoryListRequest{Q: "counter"})
	require.NoError(t, err)
	require.Len(t, list.Body, 1)
	assert.Equal(t, "kitchen_lights", list.Body[0].Key)

	// Forgotten, so its file is deleted
	_, err = m.Forget(ctx, "kitchen_lights")
	require.NoError(t, err)
	_, err = os.Stat(filepath.Join(dir, "memory-kitchen_lights.md"))
	assert.ErrorIs(t, err, os.ErrNotExist)
	assert.ErrorIs(t, store.DeleteMemory(ctx, "kitchen_lights"), llm.ErrNotFound)
	_, err = store.GetMemory(ctx, "../etc")
	assert.ErrorIs(t, err, llm.ErrNotFound)
}
