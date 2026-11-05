//go:build !client

package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	// Packages
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

func Test_JSONFile(t *testing.T) {
	t.Run("Values", func(t *testing.T) {
		// Values are kept in the file, which only the user can read, and a
		// value which is removed isn't, and the others are kept
		dir := t.TempDir()
		path := filepath.Join(dir, "sub", "values.json")
		file, err := newJSONFile(path)
		require.NoError(t, err)
		assert.False(t, file.Exists())
		assert.Nil(t, file.Get("a"))
		require.NoError(t, file.Set("a", 1))
		require.NoError(t, file.Set("b", []string{"x"}))
		require.NoError(t, file.Set("a", nil))
		assert.True(t, file.Exists())
		info, err := os.Stat(path)
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())

		again, err := newJSONFile(path)
		require.NoError(t, err)
		assert.Nil(t, again.Get("a"))
		assert.Equal(t, []any{"x"}, again.Get("b"))
		entries, err := os.ReadDir(filepath.Dir(path))
		require.NoError(t, err)
		assert.Len(t, entries, 1, "no temporary files are left")
	})

	t.Run("Shared", func(t *testing.T) {
		// A value is read from the file each time, so what another writer
		// wrote isn't replaced
		path := filepath.Join(t.TempDir(), "values.json")
		a, err := newJSONFile(path)
		require.NoError(t, err)
		b, err := newJSONFile(path)
		require.NoError(t, err)
		require.NoError(t, a.Set("a", "1"))
		require.NoError(t, b.Set("b", "2"))
		assert.Equal(t, "1", a.Get("a"))
		assert.Equal(t, "2", a.Get("b"))
	})

	t.Run("NoPath", func(t *testing.T) {
		_, err := newJSONFile(" ")
		assert.Error(t, err)
	})
}

func Test_MigrateProviders(t *testing.T) {
	ctx := context.Background()
	newFile := func(t *testing.T) *jsonFile {
		t.Helper()
		file, err := newJSONFile(filepath.Join(t.TempDir(), providersFile))
		require.NoError(t, err)
		return file
	}

	t.Run("Moved", func(t *testing.T) {
		// The providers which the defaults have are moved to the file, and
		// removed from the defaults, and those added later aren't lost when
		// a client writes the defaults it read earlier
		d := &memDefaults{}
		require.NoError(t, d.Set(providersKey, []any{map[string]any{"name": "chat", "provider": "eliza", "serial": 1}}))
		client := &memDefaults{}
		client.values = map[string]any{"llm.model": "eliza", providersKey: d.Get(providersKey)}
		file := newFile(t)
		require.NoError(t, migrateProviders(file, d))
		assert.Nil(t, d.Get(providersKey))

		store, err := newProviderStore(file)
		require.NoError(t, err)
		_, err = store.Put(ctx, chat("other", 0))
		require.NoError(t, err)
		require.NoError(t, client.Set("llm.model", "other"))
		list, err := store.List(ctx)
		require.NoError(t, err)
		require.Len(t, list, 2)
		assert.Equal(t, "chat", list[0].Name)
		assert.Equal(t, "other", list[1].Name)
	})

	t.Run("Kept", func(t *testing.T) {
		// A file which exists isn't replaced by the defaults' providers,
		// which are removed, such as when a client wrote them back
		file := newFile(t)
		store, err := newProviderStore(file)
		require.NoError(t, err)
		_, err = store.Put(ctx, chat("kept", 0))
		require.NoError(t, err)
		d := &memDefaults{}
		require.NoError(t, d.Set(providersKey, []any{map[string]any{"name": "stale", "provider": "eliza", "serial": 1}}))
		require.NoError(t, migrateProviders(file, d))
		assert.Nil(t, d.Get(providersKey))
		list, err := store.List(ctx)
		require.NoError(t, err)
		require.Len(t, list, 1)
		assert.Equal(t, "kept", list[0].Name)

		// Without providers in the defaults, there's nothing to move
		assert.NoError(t, migrateProviders(file, &memDefaults{}))
	})
}
