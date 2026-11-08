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

func Test_ChatMessage(t *testing.T) {
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
	attachment := func(name, mimeType, content string) messaging.Attachment {
		return messaging.Attachment{Name: name, Type: mimeType, Open: func(context.Context) (io.ReadCloser, error) {
			return io.NopCloser(strings.NewReader(content)), nil
		}}
	}

	// The files are kept as the session's, as they're downloaded, and
	// attached to the message, before its text
	message, err := chatMessage(ctx, res, session, messaging.Message{Text: " Our trip ", Attachments: []messaging.Attachment{
		attachment("photo.jpg", "image/jpeg", "JPEG"),
		attachment("voice.ogg", "audio/ogg", "OggS"),
	}})
	require.NoError(t, err)
	assert.Equal(t, "user", message.Role())
	assert.Equal(t, "Our trip", message.Text())
	require.Len(t, message.Content, 3)
	for i, want := range []struct{ name, mimeType, content string }{{"photo.jpg", "image/jpeg", "JPEG"}, {"voice.ogg", "audio/ogg", "OggS"}} {
		block := message.Content[i].Resource
		require.NotNil(t, block)
		assert.Equal(t, want.mimeType, block.ContentType)
		hash, err := resourceschema.ParseURI(block.URL.String())
		require.NoError(t, err)
		kept, content, err := res.Open(ctx, resourceschema.ResourceKey{Hash: hash, Session: &session})
		require.NoError(t, err)
		data, err := io.ReadAll(content)
		content.Close()
		require.NoError(t, err)
		assert.Equal(t, want.name, kept.Name)
		assert.Equal(t, want.content, string(data))
	}

	// A file which can't be downloaded is an error
	_, err = chatMessage(ctx, res, session, messaging.Message{Attachments: []messaging.Attachment{{Name: "gone.jpg", Open: func(context.Context) (io.ReadCloser, error) {
		return nil, llm.ErrNotFound.With("telegram file")
	}}}})
	assert.ErrorIs(t, err, llm.ErrNotFound)

	// A file which wasn't received, such as as it's too large, is a note
	// to the model, so it can say so, and a photo's description is kept
	message, err = chatMessage(ctx, res, session, messaging.Message{Attachments: []messaging.Attachment{
		{Name: "film.mp4", Unavailable: "it's too large (25.0 MB)"},
		{Name: "photo.jpg", Type: "image/jpeg", Description: "A photo, which Telegram compressed", Open: func(context.Context) (io.ReadCloser, error) {
			return io.NopCloser(strings.NewReader("JPEG2")), nil
		}},
	}})
	require.NoError(t, err)
	require.Len(t, message.Content, 2)
	assert.Contains(t, message.Text(), "film.mp4 wasn't received, as it's too large (25.0 MB)")
	hash, err := resourceschema.ParseURI(message.Content[0].Resource.URL.String())
	require.NoError(t, err)
	kept, err := res.Get(ctx, resourceschema.ResourceKey{Hash: hash, Session: &session})
	require.NoError(t, err)
	assert.Equal(t, "A photo, which Telegram compressed", kept.Description)

	// Text alone is the message
	message, err = chatMessage(ctx, nil, session, messaging.Message{Text: "Hello"})
	require.NoError(t, err)
	assert.Equal(t, "Hello", message.Text())
}
