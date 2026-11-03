package registry_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	registry "github.com/mutablelogic/go-llm/registry"
	schema "github.com/mutablelogic/go-llm/registry/schema"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

func Test_GoWhisper(t *testing.T) {
	// A stub go-whisper server, with a model of its own, and one of
	// OpenAI's, which it forwards to
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path != "/api/whisper/model" {
			http.NotFound(w, r)
			return
		}
		w.Write([]byte(`[{"id":"ggml-tiny.en","path":"ggml-tiny.en.bin","owned_by":"whisper"},{"id":"whisper-1","owned_by":"openai"}]`))
	}))
	t.Cleanup(server.Close)
	r, err := registry.New()
	require.NoError(t, err)
	ctx := context.Background()

	// A provider, with a URL, whose models are go-whisper's own, which
	// transcribe, and download
	provider, err := r.CreateProvider(ctx, schema.Provider{Name: "speech", Type: llm.GoWhisper, URL: server.URL})
	require.NoError(t, err)
	assert.True(t, provider.Downloader)
	assert.True(t, provider.Transcriber)
	model, err := r.GetModel(ctx, "ggml-tiny.en")
	require.NoError(t, err)
	assert.Equal(t, "speech:ggml-tiny.en", r.Name(model))
	assert.True(t, *model.Capabilities().Transcription)
	_, err = r.GetModel(ctx, "whisper-1")
	assert.ErrorIs(t, err, llm.ErrNotFound)

	// Without an API key
	_, err = r.CreateProvider(ctx, schema.Provider{Name: "keyed", Type: llm.GoWhisper, URL: server.URL, APIKey: "secret"})
	assert.ErrorIs(t, err, llm.ErrBadParameter)
}
