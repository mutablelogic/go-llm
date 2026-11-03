package httpclient_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	httpclient "github.com/mutablelogic/go-llm/registry/httpclient"
	schema "github.com/mutablelogic/go-llm/registry/schema"
	test "github.com/mutablelogic/go-llm/registry/test"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

// newTranscribeClient returns a client of a test server of a registry with
// a stub OpenAI-compatible server, as speech, with a model which
// transcribes speech
func newTranscribeClient(t *testing.T) *httpclient.Client {
	t.Helper()
	openai := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/models":
			w.Write([]byte(`{"object":"list","data":[{"id":"whisper-1","object":"model"}]}`))
		case "/v1/audio/transcriptions":
			w.Write([]byte(`{"text":"Bonjour.","language":"french","usage":{"type":"duration","seconds":2}}`))
		case "/v1/audio/translations":
			w.Write([]byte(`{"text":"Hello.","language":"english","usage":{"type":"duration","seconds":2}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(openai.Close)
	r := test.NewRegistry(t, nil)
	_, err := r.CreateProvider(t.Context(), schema.Provider{Name: "speech", Type: llm.OpenAI, URL: openai.URL})
	require.NoError(t, err)
	client, err := httpclient.New(test.NewServer(t, r).URL + test.Prefix)
	require.NoError(t, err)
	return client
}

func Test_ClientTranscribe(t *testing.T) {
	ctx := t.Context()
	client := newTranscribeClient(t)
	wav := []byte("RIFF\x24\x00\x00\x00WAVEfmt ")
	req := schema.TranscribeRequest{Model: "whisper-1", Type: "audio/wav", Data: wav}

	t.Run("Transcribe", func(t *testing.T) {
		response, err := client.Transcribe(ctx, req, nil)
		require.NoError(t, err)
		assert.Equal(t, "speech:whisper-1", response.Model)
		assert.Equal(t, "Bonjour.", response.Text)
		assert.Equal(t, "fr", response.Language)
		assert.Equal(t, uint(2), response.Usage.Seconds)
	})

	t.Run("Stream", func(t *testing.T) {
		// The text is streamed, and then the transcript
		var text string
		response, err := client.Transcribe(ctx, req, func(piece string, segment *llm.TranscriptSegment) {
			assert.Nil(t, segment)
			text += piece
		})
		require.NoError(t, err)
		assert.Equal(t, "Bonjour.", text)
		assert.Equal(t, "Bonjour.", response.Text)

		// And an error, as an error event
		_, err = client.Translate(ctx, schema.TranscribeRequest{Model: "missing", Type: "audio/wav", Data: wav}, func(string, *llm.TranscriptSegment) {})
		assert.ErrorContains(t, err, "missing")
	})

	t.Run("Translate", func(t *testing.T) {
		response, err := client.Translate(ctx, req, nil)
		require.NoError(t, err)
		assert.Equal(t, "Hello.", response.Text)
		assert.Equal(t, "en", response.Language)
	})

	t.Run("Errors", func(t *testing.T) {
		// A request needs a model, a type, and data, before it's sent, and
		// a model which exists
		for _, req := range []schema.TranscribeRequest{
			{Type: "audio/wav", Data: wav},
			{Model: "whisper-1", Data: wav},
			{Model: "whisper-1", Type: "audio/wav"},
		} {
			_, err := client.Transcribe(ctx, req, nil)
			assert.ErrorIs(t, err, llm.ErrBadParameter)
			_, err = client.Translate(ctx, req, nil)
			assert.ErrorIs(t, err, llm.ErrBadParameter)
		}
		_, err := client.Transcribe(ctx, schema.TranscribeRequest{Model: "missing", Type: "audio/wav", Data: wav}, nil)
		assert.Error(t, err)
	})
}
