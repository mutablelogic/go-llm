package ollama

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
	// The text is spoken with Ollama's OpenAI-compatible API, beside its own,
	// under the same prefix, with the options
	var path string
	var body speechRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		if body.Voice == "nobody" {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = io.WriteString(w, `{"error":{"message":"voice \"nobody\" not found"}}`)
			return
		}
		w.Header().Set("Content-Type", "audio/wav")
		_, _ = io.WriteString(w, "RIFF....WAVE")
	}))
	t.Cleanup(server.Close)
	c, err := New(server.URL + "/ollama/api")
	require.NoError(t, err)
	model := &Model{provider: c, data: apiModel{Name: "phonon:latest"}}

	audio, _, err := c.Speak(context.Background(), model, "Hello", llm.WithOpt(llm.VoiceKey, "Freya"), llm.WithOpt(llm.AudioFormatKey, "wav"))
	require.NoError(t, err)
	assert.Equal(t, "/ollama/v1/audio/speech", path)
	assert.Equal(t, speechRequest{Model: "phonon:latest", Input: "Hello", Voice: "Freya", ResponseFormat: "wav"}, body)
	assert.Equal(t, "audio/wav", audio.Type())
	data, err := llm.ReadAll(context.Background(), audio)
	require.NoError(t, err)
	assert.Equal(t, "RIFF....WAVE", string(data))

	// PCM is audio/L16, with its rate, and channels
	audio, _, err = c.Speak(context.Background(), model, "Hello", llm.WithOpt(llm.AudioFormatKey, "pcm"))
	require.NoError(t, err)
	assert.Equal(t, "audio/l16; channels=1; rate=24000", audio.Type())

	// A voice the model hasn't is an error, as are options which aren't for
	// speech, and no text
	_, _, err = c.Speak(context.Background(), model, "Hello", llm.WithOpt(llm.VoiceKey, "nobody"))
	assert.Error(t, err)
	_, _, err = c.Speak(context.Background(), model, "Hello", llm.WithOpt(llm.TemperatureKey, 0.5))
	assert.ErrorIs(t, err, llm.ErrBadParameter)
	_, _, err = c.Speak(context.Background(), model, " ")
	assert.ErrorIs(t, err, llm.ErrBadParameter)
}
