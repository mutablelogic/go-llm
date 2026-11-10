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
	require.Len(t, voices.Body, 30)
	assert.Equal(t, uint(30), voices.Count)
	assert.Equal(t, llm.Voice{ID: "Zephyr", Title: "Zephyr", Description: "Bright"}, voices.Body[0])
	assert.Equal(t, llm.Voice{ID: "Kore", Title: "Kore", Description: "Firm"}, voices.Body[3])

	// A page at a time, with the token of the next, which is empty, after
	// the last
	page, err := c.ListVoices(ctx, tts, llm.WithLimit(10))
	require.NoError(t, err)
	require.Len(t, page.Body, 10)
	assert.Equal(t, uint(30), page.Count, "of every page")
	assert.NotEmpty(t, page.Next)
	page, err = c.ListVoices(ctx, tts, llm.WithLimit(25), llm.WithPageToken(page.Next))
	require.NoError(t, err)
	require.Len(t, page.Body, 20)
	assert.Equal(t, "Iapetus", page.Body[1].ID)
	assert.Empty(t, page.Next)
	_, err = c.ListVoices(ctx, tts, llm.WithOpt(llm.VoiceKey, "Kore"))
	assert.ErrorIs(t, err, llm.ErrBadParameter, "an option which isn't for a list")

	// A model which doesn't speak has none
	_, err = c.ListVoices(ctx, chat)
	assert.ErrorIs(t, err, llm.ErrNotImplemented)
}
