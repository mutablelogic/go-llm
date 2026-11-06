//go:build !client

package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"uuid"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	resource "github.com/mutablelogic/go-llm/resource"
	schema "github.com/mutablelogic/go-llm/resource/schema"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

func Test_ResourceStore(t *testing.T) {
	ctx := t.Context()
	dir := t.TempDir()

	// A temporary file of content which wasn't written whole is removed,
	// when the store is opened
	require.NoError(t, os.MkdirAll(filepath.Join(dir, contentDir), 0o700))
	partial := filepath.Join(dir, contentDir, contentTemp+"123")
	require.NoError(t, os.WriteFile(partial, []byte("part"), 0o600))
	store, err := newResourceStore(dir)
	require.NoError(t, err)
	_, err = os.Stat(partial)
	assert.True(t, os.IsNotExist(err))

	// Content is kept once, by its hash, and read
	hash, size, err := store.Write(ctx, strings.NewReader("hello"))
	require.NoError(t, err)
	assert.Equal(t, "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824", hash)
	assert.Equal(t, int64(5), size)
	again, _, err := store.Write(ctx, strings.NewReader("hello"))
	require.NoError(t, err)
	assert.Equal(t, hash, again)
	contents, err := store.Contents(ctx)
	require.NoError(t, err)
	assert.Equal(t, []string{hash}, contents)
	content, err := store.Open(ctx, hash)
	require.NoError(t, err)
	data, err := io.ReadAll(content)
	require.NoError(t, err)
	require.NoError(t, content.Close())
	assert.Equal(t, "hello", string(data))

	// A resource, on its own, and in a session, with the one content
	session := uuid.New()
	now := time.Now().UTC().Truncate(time.Second)
	alone := schema.Resource{Hash: hash, Size: size, Created: now, Used: now, ResourceMeta: schema.ResourceMeta{Name: "a.txt", Type: "text/plain"}}
	inSession := alone
	inSession.Session = &session
	require.NoError(t, store.Create(ctx, alone))
	require.NoError(t, store.Create(ctx, inSession))
	assert.ErrorIs(t, store.Create(ctx, alone), llm.ErrConflict)
	got, err := store.Get(ctx, inSession.Key())
	require.NoError(t, err)
	assert.Equal(t, inSession, got)
	list, err := store.List(ctx)
	require.NoError(t, err)
	assert.Len(t, list, 2)

	// Changed, apart from its key, and deleted
	updated, err := store.Update(ctx, alone.Key(), func(r *schema.Resource) error {
		r.Name, r.Session = "b.txt", &session
		return nil
	})
	require.NoError(t, err)
	assert.Equal(t, "b.txt", updated.Name)
	assert.Nil(t, updated.Session, "its key isn't changed")
	_, err = store.Delete(ctx, alone.Key())
	require.NoError(t, err)
	_, err = store.Get(ctx, alone.Key())
	assert.ErrorIs(t, err, llm.ErrNotFound)

	// Content is removed
	require.NoError(t, store.Remove(ctx, hash))
	_, err = store.Open(ctx, hash)
	assert.ErrorIs(t, err, llm.ErrNotFound)
	require.NoError(t, store.Remove(ctx, hash), "removing it again does nothing")
}

func Test_ResourceStoreResources(t *testing.T) {
	// The resources, with the store, keep their content, and are cleaned,
	// and kept when the store is opened again
	ctx := t.Context()
	dir := t.TempDir()
	store, err := newResourceStore(dir)
	require.NoError(t, err)
	r, err := resource.New(store)
	require.NoError(t, err)
	created, _, err := r.Create(ctx, schema.ResourceMeta{Name: "notes.txt"}, strings.NewReader("notes"))
	require.NoError(t, err)
	r.Clean(ctx, nil)

	store, err = newResourceStore(dir)
	require.NoError(t, err)
	r, err = resource.New(store)
	require.NoError(t, err)
	got, err := r.Resource(ctx, created.URI(), nil)
	require.NoError(t, err)
	data, err := llm.ReadAll(ctx, got)
	require.NoError(t, err)
	assert.Equal(t, "notes", string(data))
}
