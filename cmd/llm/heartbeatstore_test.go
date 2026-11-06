//go:build !client

package main

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
	"uuid"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/heartbeat/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

func Test_HeartbeatStore(t *testing.T) {
	ctx := t.Context()
	dir := t.TempDir()
	store, err := newHeartbeatStore(dir)
	require.NoError(t, err)
	h := schema.Heartbeat{ID: uuid.New(), HeartbeatMeta: schema.HeartbeatMeta{Title: "Briefing", Prompt: "Brief me", Session: uuid.New(), Schedule: "0 8 * * *"}, Created: time.Now().UTC(), Next: types.Ptr(time.Now())}

	// Created, in a file of its own, which only the user can read, without
	// when it fires next, which is worked out
	require.NoError(t, store.Create(ctx, h))
	assert.ErrorIs(t, store.Create(ctx, h), llm.ErrConflict)
	info, err := os.Stat(filepath.Join(dir, "heartbeat-"+h.ID.String()+".json"))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())

	// Kept, when the store is made again
	store, err = newHeartbeatStore(dir)
	require.NoError(t, err)
	got, err := store.Get(ctx, h.ID)
	require.NoError(t, err)
	assert.Equal(t, "Briefing", got.Title)
	assert.Nil(t, got.Next)
	list, err := store.List(ctx)
	require.NoError(t, err)
	assert.Len(t, list, 1)

	// Updated, by one of two stores, which share the directory, at once
	other, err := newHeartbeatStore(dir)
	require.NoError(t, err)
	var wg sync.WaitGroup
	var fired int
	var mu sync.Mutex
	for _, s := range []*heartbeatStore{store, other, store, other} {
		wg.Go(func() {
			_, err := s.Update(ctx, h.ID, func(h *schema.Heartbeat) error {
				if h.Fired != nil {
					return llm.ErrConflict
				}
				h.Fired = types.Ptr(time.Now().UTC())
				return nil
			})
			if err == nil {
				mu.Lock()
				fired++
				mu.Unlock()
			}
		})
	}
	wg.Wait()
	assert.Equal(t, 1, fired)

	// Not deleted when the check fails, and then deleted
	_, err = store.Delete(ctx, h.ID, func(schema.Heartbeat) error { return llm.ErrConflict })
	assert.ErrorIs(t, err, llm.ErrConflict)
	_, err = store.Get(ctx, h.ID)
	require.NoError(t, err)
	_, err = store.Delete(ctx, h.ID, nil)
	require.NoError(t, err)
	_, err = store.Get(ctx, h.ID)
	assert.ErrorIs(t, err, llm.ErrNotFound)
	_, err = store.Delete(ctx, h.ID, nil)
	assert.ErrorIs(t, err, llm.ErrNotFound)
}
