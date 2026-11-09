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
	registry "github.com/mutablelogic/go-llm/registry"
	resource "github.com/mutablelogic/go-llm/resource"
	resourceschema "github.com/mutablelogic/go-llm/resource/schema"
	toolschema "github.com/mutablelogic/go-llm/toolkit/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
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
	message, err := chatMessage(ctx, res, schema.Channel{ChannelMeta: schema.ChannelMeta{Session: session}}, messaging.Message{Text: " Our trip ", Caption: true, Attachments: []messaging.Attachment{
		attachment("photo.jpg", "image/jpeg", "JPEG"),
		attachment("voice.ogg", "audio/ogg", "OggS"),
	}})
	require.NoError(t, err)
	assert.Equal(t, "user", message.Role())
	require.Len(t, message.Content, 4)
	assert.Equal(t, "Our trip", types.Value(message.Content[2].Text))

	// The text, which is the files' caption, is said to be, by their names,
	// and URIs
	reminder := types.Value(message.Content[3].Annotation)
	assert.Contains(t, reminder, "The person's text is the caption of photo.jpg (resource:///")
	assert.Contains(t, reminder, " and voice.ogg (resource:///")
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
	_, err = chatMessage(ctx, res, schema.Channel{ChannelMeta: schema.ChannelMeta{Session: session}}, messaging.Message{Attachments: []messaging.Attachment{{Name: "gone.jpg", Open: func(context.Context) (io.ReadCloser, error) {
		return nil, llm.ErrNotFound.With("telegram file")
	}}}})
	assert.ErrorIs(t, err, llm.ErrNotFound)

	// A file which wasn't received, such as one which is too large, is a note
	// to the model, so it can say so, and a photo's description is kept
	message, err = chatMessage(ctx, res, schema.Channel{ChannelMeta: schema.ChannelMeta{Session: session}}, messaging.Message{Attachments: []messaging.Attachment{
		{Name: "film.mp4", Unavailable: "it's too large (25.0 MB)"},
		{Name: "photo.jpg", Type: "image/jpeg", Description: "A photo, which Telegram compressed", Open: func(context.Context) (io.ReadCloser, error) {
			return io.NopCloser(strings.NewReader("JPEG2")), nil
		}},
	}})
	require.NoError(t, err)
	require.Len(t, message.Content, 2)
	assert.Contains(t, message.Annotations(), "film.mp4 wasn't received, as it's too large (25.0 MB)")
	hash, err := resourceschema.ParseURI(message.Content[0].Resource.URL.String())
	require.NoError(t, err)
	kept, err := res.Get(ctx, resourceschema.ResourceKey{Hash: hash, Session: &session})
	require.NoError(t, err)
	assert.Equal(t, "A photo, which Telegram compressed", kept.Description)

	// One which can't be opened, without a reason, says it couldn't be
	message, err = chatMessage(ctx, res, schema.Channel{ChannelMeta: schema.ChannelMeta{Session: session}}, messaging.Message{Text: "Here", Attachments: []messaging.Attachment{{Name: "lost.jpg"}}})
	require.NoError(t, err)
	assert.Contains(t, message.Annotations(), "lost.jpg wasn't received, as it couldn't be downloaded")

	// Text which isn't a caption isn't said to be one
	message, err = chatMessage(ctx, res, schema.Channel{ChannelMeta: schema.ChannelMeta{Session: session}}, messaging.Message{Text: "Look", Attachments: []messaging.Attachment{
		{Name: "a.jpg", Type: "image/jpeg", Open: func(context.Context) (io.ReadCloser, error) {
			return io.NopCloser(strings.NewReader("JPEG3")), nil
		}},
	}})
	require.NoError(t, err)
	assert.NotContains(t, message.Annotations(), "caption")

	// Text alone is the message
	message, err = chatMessage(ctx, nil, schema.Channel{ChannelMeta: schema.ChannelMeta{Session: session}}, messaging.Message{Text: "Hello"})
	require.NoError(t, err)
	assert.Equal(t, "Hello", message.Text())
}

func Test_ReplyFiles(t *testing.T) {
	// A reply's media are sent with it, and files a model reads with, such
	// as a transcript, aren't
	reply := registry.Reply{Text: "Here you are", Resources: []toolschema.ResourceLink{
		{URI: "resource:///speech", Type: "audio/ogg"},
		{URI: "resource:///transcript", Type: "text/plain; charset=utf-8"},
		{URI: "resource:///picture", Type: "image/png"},
		{URI: "resource:///scan", Type: "text/markdown"},
		{URI: "resource:///film", Type: "video/mp4"},
		{URI: "resource:///report", Type: "application/pdf"},
		{URI: "resource:///unknown"},
	}}
	assert.Equal(t, []string{"resource:///speech", "resource:///picture", "resource:///film"}, replyFiles(reply))
}

func Test_ProgressStream(t *testing.T) {
	// The lines tools log, and their progress, are what's happening, and the
	// reply, and thinking, aren't
	var lines []string
	stream := progressStream(func(line string) { lines = append(lines, line) })
	stream(llm.StreamText, "Here's the answer")
	stream(llm.StreamThinking, "Hmm")
	stream(llm.StreamLog, "Transcribing the recording")
	stream(llm.StreamProgress, llm.Progress{Progress: 1, Total: 3, Message: "The segment agent is replying"}.Text())
	stream(llm.StreamProgress, llm.Progress{Progress: 2}.Text())
	assert.Equal(t, []string{"Transcribing the recording", "The segment agent is replying"}, lines)
	assert.Nil(t, progressStream(nil))
}

func Test_ChatMessageWithoutResources(t *testing.T) {
	// Without resources, each file is a note that it wasn't received, rather
	// than the first stopping the others
	open := func(context.Context) (io.ReadCloser, error) { return io.NopCloser(strings.NewReader("x")), nil }
	message, err := chatMessage(t.Context(), nil, schema.Channel{ChannelMeta: schema.ChannelMeta{Session: uuid.New()}}, messaging.Message{Text: "Two files", Attachments: []messaging.Attachment{
		{Name: "a.jpg", Open: open},
		{Name: "b.pdf", Open: open},
	}})
	require.NoError(t, err)
	assert.Equal(t, "Two files", message.Text())
	assert.Contains(t, message.Annotations(), "a.jpg wasn't received, as files can't be kept")
	assert.Contains(t, message.Annotations(), "b.pdf wasn't received, as files can't be kept")
}

func Test_Origin(t *testing.T) {
	// Where a message was sent, and who sent it, which, in a chat with one
	// person, is the chat's name
	telegram := func(title string) schema.Channel {
		return schema.Channel{ChannelMeta: schema.ChannelMeta{Provider: "telegram", Title: title}}
	}
	assert.Equal(t, "Sent on Telegram, by David.", origin(telegram("David"), messaging.Message{Title: "David", From: "David"}))
	assert.Equal(t, `Sent on Telegram, in the chat "Family", by Anne.`, origin(telegram("Family"), messaging.Message{Title: "Family", From: "Anne"}))
	assert.Equal(t, `Sent on Telegram, in the chat "Family".`, origin(telegram("Family"), messaging.Message{}))
	assert.Empty(t, origin(schema.Channel{}, messaging.Message{From: "David"}))

	// and the session is told it, as an annotation, rather than text, so
	// it isn't shown as what the person said, before what it says of the files
	message, err := chatMessage(t.Context(), nil, telegram("Family"), messaging.Message{Title: "Family", From: "Anne", Text: "Who's cooking?"})
	require.NoError(t, err)
	require.Len(t, message.Content, 2)
	assert.Equal(t, "Who's cooking?", types.Value(message.Content[0].Text))
	assert.Nil(t, message.Content[1].Text)
	assert.Equal(t, `Sent on Telegram, in the chat "Family", by Anne.`, types.Value(message.Content[1].Annotation))
	assert.Equal(t, "Who's cooking?", message.Text())
}
