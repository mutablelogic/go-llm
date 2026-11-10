package openai

import (
	"context"
	"testing"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	types "github.com/mutablelogic/go-server/pkg/types"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

func Test_ListVoices(t *testing.T) {
	ctx := context.Background()
	c, err := New("", "key")
	require.NoError(t, err)

	// OpenAI's newer speech models speak in its voices, of which it
	// recommends marin and cedar
	voices, err := c.ListVoices(ctx, c.Model("gpt-4o-mini-tts"))
	require.NoError(t, err)
	require.Len(t, voices.Body, 13)
	assert.Equal(t, llm.Voice{ID: "alloy", Title: "Alloy"}, voices.Body[0])
	assert.Equal(t, llm.Voice{ID: "marin", Title: "Marin", Description: "OpenAI's best quality", Recommended: types.Ptr(true)}, voices.Body[11])

	// and tts-1 and tts-1-hd in fewer
	for _, name := range []string{"tts-1", "tts-1-hd"} {
		voices, err := c.ListVoices(ctx, c.Model(name))
		require.NoError(t, err)
		assert.Len(t, voices.Body, 9, name)
	}

	// A model which isn't known to speak has none
	for _, name := range []string{"whisper-1", "text-embedding-3-small", "gpt-4o"} {
		voices, err := c.ListVoices(ctx, c.Model(name))
		require.NoError(t, err, name)
		assert.Empty(t, voices.Body, name)
	}

	// Another OpenAI-compatible server doesn't say
	other, err := New("http://localhost:8000/v1", "")
	require.NoError(t, err)
	voices, err = other.ListVoices(ctx, other.Model("kokoro"))
	require.NoError(t, err)
	assert.Empty(t, voices.Body)

	// A model of another client isn't listed
	_, err = other.ListVoices(ctx, c.Model("tts-1"))
	assert.ErrorIs(t, err, llm.ErrBadParameter)
}
