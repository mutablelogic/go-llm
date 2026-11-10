package gemini

import (
	"context"
	"testing"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

func Test_ListVoices(t *testing.T) {
	ctx := context.Background()
	_, c, chat := chatClient(t)

	// A speech model speaks in the prebuilt voices, each with its style
	tts, err := c.GetModel(ctx, "gemini-new-preview-tts")
	require.NoError(t, err)
	voices, err := c.ListVoices(ctx, tts)
	require.NoError(t, err)
	require.Len(t, voices, 30)
	assert.Equal(t, llm.Voice{ID: "Zephyr", Title: "Zephyr", Description: "Bright"}, voices[0])
	assert.Equal(t, llm.Voice{ID: "Kore", Title: "Kore", Description: "Firm"}, voices[3])

	// A model which doesn't speak has none
	_, err = c.ListVoices(ctx, chat)
	assert.ErrorIs(t, err, llm.ErrNotImplemented)
}
