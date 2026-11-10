package openai

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
	c, err := New("", "key")
	require.NoError(t, err)

	// OpenAI's newer speech models speak in its voices, of which it
	// recommends marin and cedar
	voices, _, err := c.ListVoices(ctx, c.Model("gpt-4o-mini-tts"))
	require.NoError(t, err)
	require.Len(t, voices, 13)
	assert.Equal(t, llm.Voice{ID: "alloy", Title: "Alloy"}, voices[0])
	assert.Equal(t, llm.Voice{ID: "marin", Title: "Marin", Description: "Recommended, for the best quality"}, voices[11])

	// and tts-1 and tts-1-hd in fewer
	for _, name := range []string{"tts-1", "tts-1-hd"} {
		voices, _, err := c.ListVoices(ctx, c.Model(name))
		require.NoError(t, err)
		assert.Len(t, voices, 9, name)
	}

	// A model which doesn't speak has none
	_, _, err = c.ListVoices(ctx, c.Model("whisper-1"))
	assert.NoError(t, err, "whisper-1 isn't known not to speak")
	_, _, err = c.ListVoices(ctx, c.Model("text-embedding-3-small"))
	assert.NoError(t, err)

	// Another OpenAI-compatible server doesn't say
	other, err := New("http://localhost:8000/v1", "")
	require.NoError(t, err)
	voices, _, err = other.ListVoices(ctx, other.Model("kokoro"))
	require.NoError(t, err)
	assert.Empty(t, voices)

	// A model of another client isn't listed
	_, _, err = other.ListVoices(ctx, c.Model("tts-1"))
	assert.ErrorIs(t, err, llm.ErrBadParameter)
}
