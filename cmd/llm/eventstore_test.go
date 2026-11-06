//go:build !client

package main

import (
	"os"
	"path/filepath"
	"testing"
	"uuid"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/registry/schema"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

func Test_EventStore(t *testing.T) {
	ctx := t.Context()
	dir := t.TempDir()
	store, err := newEventStore(dir)
	require.NoError(t, err)
	session := uuid.New()

	// Created, in a file of its own, which only the user can read
	first, err := store.Create(ctx, schema.EventMeta{Run: uuid.New(), Session: session, Message: "Call your mother", Importance: schema.ImportanceAlert, Status: schema.EventPending})
	require.NoError(t, err)
	assert.Equal(t, uint64(1), first.Serial)
	info, err := os.Stat(filepath.Join(dir, "event-"+first.ID.String()+".json"))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
	second, err := store.Create(ctx, schema.EventMeta{Run: uuid.New(), Session: uuid.New(), Message: "Nothing new", Importance: schema.ImportanceIgnore, Status: schema.EventIgnored})
	require.NoError(t, err)

	// Kept, when the store is made again, and listed, the most recent first,
	// and by session, and status
	store, err = newEventStore(dir)
	require.NoError(t, err)
	list, err := store.List(ctx, schema.EventListRequest{})
	require.NoError(t, err)
	require.Equal(t, uint(2), list.Count)
	assert.Equal(t, second.ID, list.Body[0].ID)
	list, err = store.List(ctx, schema.EventListRequest{Session: session.String()})
	require.NoError(t, err)
	require.Equal(t, uint(1), list.Count)
	assert.Equal(t, first.ID, list.Body[0].ID)
	list, err = store.List(ctx, schema.EventListRequest{Status: schema.EventIgnored})
	require.NoError(t, err)
	require.Equal(t, uint(1), list.Count)
	assert.Equal(t, second.ID, list.Body[0].ID)

	// Changed, at its serial, and not, at another
	first.Status = schema.EventDelivered
	delivered, err := store.Put(ctx, first)
	require.NoError(t, err)
	assert.Equal(t, uint64(2), delivered.Serial)
	assert.Equal(t, schema.EventDelivered, delivered.Status)
	_, err = store.Put(ctx, first)
	assert.ErrorIs(t, err, llm.ErrConflict)

	// Deleted, at its serial
	assert.ErrorIs(t, store.Delete(ctx, first.ID, 1), llm.ErrConflict)
	require.NoError(t, store.Delete(ctx, first.ID, delivered.Serial))
	_, err = store.Get(ctx, first.ID)
	assert.ErrorIs(t, err, llm.ErrNotFound)
	assert.NoError(t, store.Delete(ctx, first.ID, delivered.Serial), "nothing to delete")
}
