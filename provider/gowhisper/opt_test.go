package gowhisper

import (
	"encoding/json"
	"testing"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

func Test_TranscribeApply(t *testing.T) {
	t.Run("Options", func(t *testing.T) {
		// The language, temperature and segments
		var r transcribeRequest
		options, err := r.apply("ggml-medium-q5_0", false, llm.WithLanguage("fr"), llm.WithTemperature(0.2), llm.WithSegments(true), llm.WithSpeakers(false))
		require.NoError(t, err)
		assert.Equal(t, "fr", r.Language)
		assert.Equal(t, 0.2, *r.Temperature)
		assert.Equal(t, transcribeOptions{language: "fr", segments: true}, options)
	})

	t.Run("Errors", func(t *testing.T) {
		// A translation has no language, a language is a code, speakers
		// aren't distinguished, and other options aren't valid
		for _, test := range []struct {
			translate bool
			opt       llm.Opt
		}{
			{true, llm.WithLanguage("fr")},
			{false, llm.WithOpt(llm.LanguageKey, json.RawMessage(`"French"`))},
			{false, llm.WithSpeakers(true)},
			{false, llm.WithDimensions(3)},
		} {
			var r transcribeRequest
			_, err := r.apply("ggml-medium-q5_0", test.translate, test.opt)
			assert.ErrorIs(t, err, llm.ErrBadParameter)
		}
	})
}
