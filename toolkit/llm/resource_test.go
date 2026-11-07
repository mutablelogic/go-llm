package llm_test

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	imagepng "image/png"
	"strings"
	"testing"
	"time"
	"uuid"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	resource "github.com/mutablelogic/go-llm/resource"
	schema "github.com/mutablelogic/go-llm/resource/schema"
	toolkit "github.com/mutablelogic/go-llm/toolkit"
	llmtools "github.com/mutablelogic/go-llm/toolkit/llm"
	types "github.com/mutablelogic/go-server/pkg/types"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

const srt = "1\n00:00:00,200 --> 00:00:03,500\nThe quick brown fox jumps over the lazy dog.\n\n2\n00:00:04,800 --> 00:00:05,600\nGood morning.\n"

func Test_Resource(t *testing.T) {
	ctx := context.Background()
	manager, err := resource.New(resource.NewStore())
	require.NoError(t, err)
	ns, err := llmtools.New(func() llmtools.Models { return &models{} }, llmtools.WithResources(manager))
	require.NoError(t, err)
	var read llm.Tool
	for _, tool := range ns.Tools() {
		if tool.Name() == "resource" {
			read = tool
		}
	}
	require.NotNil(t, read, "with resources, there's a resource tool")
	assert.Equal(t, time.Minute, read.Timeout())

	// Without, there's none
	without, err := llmtools.New(func() llmtools.Models { return &models{} })
	require.NoError(t, err)
	for _, tool := range without.Tools() {
		assert.NotEqual(t, "resource", tool.Name())
	}

	// A chat's files, and one on its own
	chat := uuid.New()
	create := func(meta schema.ResourceMeta, content string) string {
		t.Helper()
		file, _, err := manager.Create(ctx, meta, strings.NewReader(content))
		require.NoError(t, err)
		return file.URI()
	}
	notes := create(schema.ResourceMeta{Session: &chat, Name: "notes.txt", Type: "text/plain", Description: "Shopping | list\nfor Saturday"}, "Eggs, milk, and bread.\nAnd coffee.")
	subtitles := create(schema.ResourceMeta{Session: &chat, Name: "talk.srt", Type: "application/x-subrip"}, srt)
	audio := create(schema.ResourceMeta{Session: &chat, Name: "talk.mp3", Type: "audio/mpeg"}, "ID3xxxxxxxx")
	alone := create(schema.ResourceMeta{Name: "data.csv", Type: "text/csv"}, "name,age\nAnna,42\n")
	var png bytes.Buffer
	require.NoError(t, imagepng.Encode(&png, image.NewRGBA(image.Rect(0, 0, 64, 48))))
	picture := create(schema.ResourceMeta{Session: &chat, Name: "dot.png", Type: "image/png"}, png.String())

	// run calls the tool, in the chat, or without one
	run := func(session uuid.UUID, input string) (any, error) {
		ctx := toolkit.WithSession(ctx, toolkit.NewSession("llm", session, func(llm.StreamType, string) {}, nil))
		return read.Run(ctx, json.RawMessage(input))
	}
	output := func(result any) map[string]any {
		t.Helper()
		data, err := json.Marshal(result)
		require.NoError(t, err)
		var got map[string]any
		require.NoError(t, json.Unmarshal(data, &got))
		return got
	}

	t.Run("List", func(t *testing.T) {
		// Without a URI, the chat's files, as a Markdown table, with their
		// titles on one line, without the pipes which would end a cell
		result, err := run(chat, `{}`)
		require.NoError(t, err)
		table := result.(string)
		assert.Contains(t, table, "This chat has 4 files:")
		assert.Contains(t, table, "| Name | Title | Type | Size | URI |")
		assert.Contains(t, table, "| notes.txt | Shopping \\| list | text/plain | 34 bytes | "+notes+" |")
		assert.Contains(t, table, subtitles)
		assert.Contains(t, table, audio)
		assert.NotContains(t, table, alone, "a file on its own isn't the chat's")

		// Another chat has none, and there's nothing to list without one
		result, err = run(uuid.New(), `{}`)
		require.NoError(t, err)
		assert.Equal(t, "This chat has no files.", result)
		_, err = run(uuid.Nil(), `{}`)
		assert.ErrorIs(t, err, llm.ErrBadParameter)
	})

	t.Run("Text", func(t *testing.T) {
		// A text file, as Markdown, with what it is
		result, err := run(chat, `{"uri":"`+notes+`"}`)
		require.NoError(t, err)
		got := output(result)
		assert.Equal(t, notes, got["uri"])
		assert.Equal(t, "notes.txt", got["name"])
		assert.Equal(t, "text/plain", got["type"])
		assert.Contains(t, got["content"], "Eggs, milk, and bread.")
		assert.Nil(t, got["next"])

		// A part at a time, from where the last part ended
		result, err = run(chat, `{"uri":"`+notes+`","limit":10}`)
		require.NoError(t, err)
		got = output(result)
		require.NotNil(t, got["next"])
		first := got["content"].(string)
		next := uint64(got["next"].(float64))
		result, err = run(chat, `{"uri":"`+notes+`","offset":`+types.Stringify(next)+`}`)
		require.NoError(t, err)
		got = output(result)
		assert.Equal(t, float64(next), got["offset"])
		assert.Contains(t, first+got["content"].(string), "And coffee.")

		// Beyond its end, which is a note, and too small a limit
		result, err = run(chat, `{"uri":"`+notes+`","offset":100000}`)
		require.NoError(t, err)
		assert.Contains(t, output(result)["note"], "beyond the end")
		_, err = run(chat, `{"uri":"`+notes+`","limit":2}`)
		assert.ErrorIs(t, err, llm.ErrBadParameter)
	})

	t.Run("Subtitles", func(t *testing.T) {
		// Subtitles, as Markdown, with what's said
		result, err := run(chat, `{"uri":"`+subtitles+`"}`)
		require.NoError(t, err)
		got := output(result)
		assert.Contains(t, got["content"], "The quick brown fox jumps over the lazy dog.")
		assert.Contains(t, got["content"], "Good morning.")
	})

	t.Run("Metadata", func(t *testing.T) {
		// An image isn't text, but has its metadata, such as its size, and
		// what it is, without what's read next, which is a handler's
		result, err := run(chat, `{"uri":"`+picture+`"}`)
		require.NoError(t, err)
		got := output(result)
		assert.Nil(t, got["content"])
		assert.Contains(t, got["note"], "can't be read as text")
		meta, ok := got["metadata"].(map[string]any)
		require.True(t, ok, "it has metadata")
		assert.Equal(t, "64", meta["image:width"])
		assert.Equal(t, "48", meta["image:height"])
		assert.Nil(t, meta["metadata:hint"])

		// A text file's is with its first part, and not the next
		result, err = run(chat, `{"uri":"`+notes+`","limit":10}`)
		require.NoError(t, err)
		got = output(result)
		assert.NotNil(t, got["metadata"])
		next := uint64(got["next"].(float64))
		result, err = run(chat, `{"uri":"`+notes+`","offset":`+types.Stringify(next)+`}`)
		require.NoError(t, err)
		assert.Nil(t, output(result)["metadata"])
	})

	t.Run("NotText", func(t *testing.T) {
		// Audio isn't text, so it's what it is, with a note, and no content
		result, err := run(chat, `{"uri":"`+audio+`"}`)
		require.NoError(t, err)
		got := output(result)
		assert.Equal(t, "audio/mpeg", got["type"])
		assert.Nil(t, got["content"])
		assert.Contains(t, got["note"], "can't be read as text")
	})

	t.Run("OnItsOwn", func(t *testing.T) {
		// A file on its own is read in a chat, and without one, and one which
		// doesn't exist, or isn't a file's URI, isn't
		for _, session := range []uuid.UUID{chat, uuid.Nil()} {
			result, err := run(session, `{"uri":"`+alone+`"}`)
			require.NoError(t, err)
			assert.Contains(t, output(result)["content"], "Anna")
		}
		_, err := run(chat, `{"uri":"resource://`+strings.Repeat("0", 64)+`"}`)
		assert.ErrorIs(t, err, llm.ErrNotFound)
		_, err = run(chat, `{"uri":"https://example.com"}`)
		assert.ErrorIs(t, err, llm.ErrBadParameter)

		// Another chat's file isn't this one's
		_, err = run(uuid.New(), `{"uri":"`+notes+`"}`)
		assert.ErrorIs(t, err, llm.ErrNotFound)
	})
}
