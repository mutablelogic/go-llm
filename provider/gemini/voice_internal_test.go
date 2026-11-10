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
	voices, _, err := c.ListVoices(ctx, tts)
	require.NoError(t, err)
	require.Len(t, voices, 30)
	assert.Equal(t, llm.Voice{ID: "Zephyr", Title: "Zephyr", Description: "Bright"}, voices[0])
	assert.Equal(t, llm.Voice{ID: "Kore", Title: "Kore", Description: "Firm"}, voices[3])

	// A page at a time, with the token of the next, which is empty, after
	// the last
	page, next, err := c.ListVoices(ctx, tts, llm.WithLimit(10))
	require.NoError(t, err)
	require.Len(t, page, 10)
	assert.Equal(t, "10", next)
	page, next, err = c.ListVoices(ctx, tts, llm.WithLimit(25), llm.WithPageToken(next))
	require.NoError(t, err)
	require.Len(t, page, 20)
	assert.Equal(t, "Iapetus", page[1].ID)
	assert.Empty(t, next)
	_, _, err = c.ListVoices(ctx, tts, llm.WithOpt(llm.VoiceKey, "Kore"))
	assert.ErrorIs(t, err, llm.ErrBadParameter, "an option which isn't for a list")

	// A model which doesn't speak has none
	_, _, err = c.ListVoices(ctx, chat)
	assert.ErrorIs(t, err, llm.ErrNotImplemented)
}
