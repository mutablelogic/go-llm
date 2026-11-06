package httpclient_test

import (
	"bytes"
	"encoding/json"
	"io"
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

func Test_Speak(t *testing.T) {
	ctx := t.Context()
	c := newSpeakClient(t)

	// The audio is written, with its type, and the model which spoke it
	var audio bytes.Buffer
	contentType, meta, err := c.Speak(ctx, schema.SpeakRequest{Model: "speaker", Text: "Hello"}, &audio)
	require.NoError(t, err)
	assert.Equal(t, "audio/wav", contentType)
	assert.Equal(t, "Hello", audio.String())
	assert.Equal(t, "speaking:speaker", meta.Model)

	// A model which doesn't speak, and no text, aren't
	_, _, err = c.Speak(ctx, schema.SpeakRequest{Model: "silent", Text: "Hello"}, &audio)
	assert.Error(t, err)
	_, _, err = c.Speak(ctx, schema.SpeakRequest{Model: "speaker"}, &audio)
	assert.ErrorIs(t, err, llm.ErrBadParameter)
}

// newSpeakClient returns a client of a server with an OpenAI-compatible
// provider, speaking, whose models are speaker, which speaks the text, and
// silent, which can't
func newSpeakClient(t *testing.T) *httpclient.Client {
	t.Helper()
	openai := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/models":
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"object":"list","data":[{"id":"speaker","object":"model"},{"id":"silent","object":"model"}]}`)
		case "/v1/audio/speech":
			var req struct {
				Model string `json:"model"`
				Input string `json:"input"`
			}
			_ = json.NewDecoder(r.Body).Decode(&req)
			if req.Model == "silent" {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusBadRequest)
				_, _ = io.WriteString(w, `{"error":{"message":"this model can't speak"}}`)
				return
			}
			w.Header().Set("Content-Type", "audio/wav")
			_, _ = io.WriteString(w, req.Input)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(openai.Close)
	r := test.NewRegistry(t, nil)
	_, err := r.CreateProvider(t.Context(), schema.Provider{Name: "speaking", Type: llm.OpenAI, URL: openai.URL})
	require.NoError(t, err)
	client, err := httpclient.New(test.NewServer(t, r).URL + test.Prefix)
	require.NoError(t, err)
	return client
}
