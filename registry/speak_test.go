package registry_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	ollama "github.com/mutablelogic/go-llm/provider/ollama"
	test "github.com/mutablelogic/go-llm/registry/test"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

func Test_ListVoices(t *testing.T) {
	// An Ollama server with a speech model, which has five voices, and a
	// model which doesn't speak
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/api/tags":
			_, _ = io.WriteString(w, `{"models":[{"name":"phonon:latest"},{"name":"llama3.2:latest"}]}`)
		case r.URL.Path == "/api/show" && strings.Contains(string(body), "phonon"):
			_, _ = io.WriteString(w, `{"capabilities":["speech"],"voices":["Alfie","Bryony","Cara","Dev","Eve"]}`)
		case r.URL.Path == "/api/show":
			_, _ = io.WriteString(w, `{"capabilities":["completion"]}`)
		case r.URL.Path == "/api/version":
			_, _ = io.WriteString(w, `{"version":"0.20.0"}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)
	provider, err := ollama.New(server.URL)
	require.NoError(t, err)
	r := test.NewRegistry(t, []llm.Provider{provider})
	ctx := context.Background()

	model, err := r.GetModel(ctx, "phonon:latest")
	require.NoError(t, err)

	// Every voice
	voices, next, err := r.ListVoices(ctx, model)
	require.NoError(t, err)
	require.Len(t, voices, 5)
	assert.Equal(t, llm.Voice{ID: "Alfie", Title: "Alfie"}, voices[0])
	assert.Empty(t, next)

	// A page at a time
	voices, next, err = r.ListVoices(ctx, model, llm.WithLimit(2))
	require.NoError(t, err)
	assert.Equal(t, []string{"Alfie", "Bryony"}, ids(voices))
	voices, next, err = r.ListVoices(ctx, model, llm.WithLimit(2), llm.WithPageToken(next))
	require.NoError(t, err)
	assert.Equal(t, []string{"Cara", "Dev"}, ids(voices))
	voices, next, err = r.ListVoices(ctx, model, llm.WithLimit(2), llm.WithPageToken(next))
	require.NoError(t, err)
	assert.Equal(t, []string{"Eve"}, ids(voices))
	assert.Empty(t, next)

	// A model which doesn't speak
	llama, err := r.GetModel(ctx, "llama3.2:latest")
	require.NoError(t, err)
	_, _, err = r.ListVoices(ctx, llama)
	assert.ErrorIs(t, err, llm.ErrNotImplemented)
}

// ids returns the voices' identifiers
func ids(voices []llm.Voice) []string {
	result := make([]string, len(voices))
	for i, voice := range voices {
		result[i] = voice.ID
	}
	return result
}
