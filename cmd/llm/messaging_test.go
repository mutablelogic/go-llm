//go:build !client

package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
	"uuid"

	// Packages
	schema "github.com/mutablelogic/go-llm/messaging/schema"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

func Test_ChannelStore(t *testing.T) {
	ctx := t.Context()
	path := filepath.Join(t.TempDir(), channelsFile)
	file, err := newJSONFile(path)
	require.NoError(t, err)
	store := &channelStore{file: file}

	// None, until they're saved
	channels, err := store.Load(ctx)
	require.NoError(t, err)
	assert.Empty(t, channels)

	// They're kept, as they were
	saved := []schema.Channel{{
		ID:          uuid.New(),
		ChannelMeta: schema.ChannelMeta{Session: uuid.New(), Provider: "telegram", Address: "-100123", Title: "Family"},
		Created:     time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC),
	}}
	require.NoError(t, store.Save(ctx, saved))
	channels, err = (&channelStore{file: file}).Load(ctx)
	require.NoError(t, err)
	assert.Equal(t, saved, channels)
	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())

	// None, once they're all gone
	require.NoError(t, store.Save(ctx, nil))
	channels, err = store.Load(ctx)
	require.NoError(t, err)
	assert.Empty(t, channels)
}
