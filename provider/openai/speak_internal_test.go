package openai

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

func Test_Speak(t *testing.T) {
	var body speechRequest
	var contentType string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/audio/speech", r.URL.Path)
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		w.Header().Set("Content-Type", contentType)
		_, _ = io.WriteString(w, "audio")
	}))
	t.Cleanup(server.Close)
	ctx := context.Background()

	// Another server is sent the options, and no voice, unless one's given,
	// and the audio's type is the response's
	c, err := New(server.URL+"/v1", "")
	require.NoError(t, err)
	model := &Model{provider: c, data: apiModel{ID: "tts-1"}}
	contentType = "audio/mpeg"
	audio, _, err := c.Speak(ctx, model, "Hello", llm.WithOpt(llm.SpeedKey, 1.5), llm.WithOpt(llm.InstructionsKey, "Calmly"))
	require.NoError(t, err)
	assert.Equal(t, "", body.Voice)
	assert.Equal(t, 1.5, *body.Speed)
	assert.Equal(t, "Calmly", body.Instructions)
	assert.Equal(t, "audio/mpeg", audio.Type())

	// A response which only says it's binary has the format's type
	contentType = "application/octet-stream"
	audio, _, err = c.Speak(ctx, model, "Hello", llm.WithOpt(llm.AudioFormatKey, "wav"))
	require.NoError(t, err)
	assert.Equal(t, "wav", body.ResponseFormat)
	assert.Equal(t, "audio/wav", audio.Type())

	// PCM, which is little-endian, is audio/L16, which is big-endian, with
	// its rate, and channels
	contentType = "audio/pcm"
	audio, _, err = c.Speak(ctx, model, "Hello", llm.WithOpt(llm.AudioFormatKey, "pcm"))
	require.NoError(t, err)
	assert.Equal(t, "audio/l16; channels=1; rate=24000", audio.Type())
	data, err := llm.ReadAll(ctx, audio)
	require.NoError(t, err)
	assert.Equal(t, "uaid", string(data), "audio is swapped in pairs, without its trailing odd byte")

	// Options which aren't for speech aren't sent
	_, _, err = c.Speak(ctx, model, "Hello", llm.WithOpt(llm.TemperatureKey, 0.5))
	assert.ErrorIs(t, err, llm.ErrBadParameter)

	// OpenAI's own models are spoken in its default voice, unless one's given
	openai, err := New(OpenAIEndpoint, "key")
	require.NoError(t, err)
	assert.Equal(t, serverOpenAI, openai.server())
	assert.True(t, *(&Model{provider: openai, data: apiModel{ID: "tts-1"}}).Capabilities().Speech)
	assert.True(t, *(&Model{provider: openai, data: apiModel{ID: "gpt-4o-mini-tts"}}).Capabilities().Speech)
}
