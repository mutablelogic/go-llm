//go:build !client

package main

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"uuid"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	messaging "github.com/mutablelogic/go-llm/messaging"
	schema "github.com/mutablelogic/go-llm/messaging/schema"
	resource "github.com/mutablelogic/go-llm/resource"
	resourceschema "github.com/mutablelogic/go-llm/resource/schema"
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

func Test_OpenFile(t *testing.T) {
	ctx := t.Context()
	session := uuid.New()
	exists := func(_ context.Context, id uuid.UUID) error {
		if id != session {
			return llm.ErrNotFound.Withf("session %q", id)
		}
		return nil
	}
	res, err := resource.New(resource.NewStore(), resource.WithSessions(exists))
	require.NoError(t, err)
	made, _, err := res.Create(ctx, resourceschema.ResourceMeta{Name: "speech.ogg", Type: "audio/ogg", Session: &session}, strings.NewReader("OggS"))
	require.NoError(t, err)

	// The session's file is opened, with what it is
	open := openFile(res)
	file, content, err := open(ctx, session, made.URI())
	require.NoError(t, err)
	defer content.Close()
	data, err := io.ReadAll(content)
	require.NoError(t, err)
	assert.Equal(t, "OggS", string(data))
	assert.Equal(t, messaging.File{URI: made.URI(), Name: "speech.ogg", Type: "audio/ogg", Size: 4}, file)

	// Another session's, or a URI which isn't one, isn't
	_, _, err = open(ctx, uuid.New(), made.URI())
	assert.ErrorIs(t, err, llm.ErrNotFound)
	_, _, err = open(ctx, session, "https://example.com")
	assert.ErrorIs(t, err, llm.ErrBadParameter)
}
