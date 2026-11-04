package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	// Packages
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

// A question about files has each file, and then the question: a file which
// is text, with its name, and any other file as an attachment, with its type
func Test_AskMessage(t *testing.T) {
	dir := t.TempDir()
	write := func(name string, data []byte) string {
		path := filepath.Join(dir, name)
		require.NoError(t, os.WriteFile(path, data, 0o644))
		return path
	}
	code := write("main, v2.go", []byte("package main\n\nfunc main() {}\n"))
	srt := write("episode.de.srt", []byte("1\n00:00:00,000 --> 00:00:02,000\nGrüß Gott.\n"))
	png := write("image.png", []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR"))
	pdf := write("doc.pdf", []byte("%PDF-1.7\n1 0 obj\n"))

	message, err := askMessage("What do these say?", []string{code, srt, png, pdf})
	require.NoError(t, err)
	require.Len(t, message.Content, 5)

	// Text files, with their names, in order
	require.NotNil(t, message.Content[0].Text)
	assert.Equal(t, "main, v2.go:\n\n```\npackage main\n\nfunc main() {}\n```", *message.Content[0].Text)
	require.NotNil(t, message.Content[1].Text)
	assert.True(t, strings.HasPrefix(*message.Content[1].Text, "episode.de.srt:\n\n```\n1\n"))
	assert.Contains(t, *message.Content[1].Text, "Grüß Gott.")

	// Attachments, with their types and data
	require.NotNil(t, message.Content[2].Resource)
	assert.Equal(t, "image/png", message.Content[2].Resource.ContentType)
	assert.Equal(t, []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR"), message.Content[2].Resource.Data)
	require.NotNil(t, message.Content[3].Resource)
	assert.Equal(t, "application/pdf", message.Content[3].Resource.ContentType)

	// And then the question
	require.NotNil(t, message.Content[4].Text)
	assert.Equal(t, "What do these say?", *message.Content[4].Text)
	assert.Len(t, message.Resources(), 2)
}

// Without files, the message is the question
func Test_AskMessage_NoFiles(t *testing.T) {
	message, err := askMessage("Hello?", nil)
	require.NoError(t, err)
	require.Len(t, message.Content, 1)
	assert.Equal(t, "Hello?", *message.Content[0].Text)
}

// A file which can't be read is an error
func Test_AskMessage_Missing(t *testing.T) {
	_, err := askMessage("Hello?", []string{filepath.Join(t.TempDir(), "missing.txt")})
	assert.ErrorIs(t, err, os.ErrNotExist)
}
