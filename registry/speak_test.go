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
	schema "github.com/mutablelogic/go-llm/registry/schema"
	test "github.com/mutablelogic/go-llm/registry/test"
	types "github.com/mutablelogic/go-server/pkg/types"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

func Test_ListModelVoices(t *testing.T) {
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

	// Every voice, and how many there are
	list, err := r.ListModelVoices(ctx, "phonon:latest", schema.VoiceListRequest{})
	require.NoError(t, err)
	assert.Equal(t, "ollama:phonon:latest", list.Model)
	assert.Equal(t, uint(5), list.Count)
	require.Len(t, list.Body, 5)
	assert.Equal(t, llm.Voice{ID: "Alfie", Title: "Alfie"}, list.Body[0])

	// From an offset, and at most a limit of them
	list, err = r.ListModelVoices(ctx, "phonon:latest", schema.VoiceListRequest{Offset: 1, Limit: types.Ptr(uint(2))})
	require.NoError(t, err)
	assert.Equal(t, uint(5), list.Count)
	assert.Equal(t, []string{"Bryony", "Cara"}, ids(list.Body))
	list, err = r.ListModelVoices(ctx, "phonon:latest", schema.VoiceListRequest{Offset: 9})
	require.NoError(t, err)
	assert.Equal(t, uint(5), list.Offset)
	assert.Empty(t, list.Body)

	// A model which doesn't speak, or isn't there
	_, err = r.ListModelVoices(ctx, "llama3.2:latest", schema.VoiceListRequest{})
	assert.ErrorIs(t, err, llm.ErrNotImplemented)
	_, err = r.ListModelVoices(ctx, "missing:latest", schema.VoiceListRequest{})
	assert.ErrorIs(t, err, llm.ErrNotFound)
}

// ids returns the voices' identifiers
func ids(voices []llm.Voice) []string {
	result := make([]string, len(voices))
	for i, voice := range voices {
		result[i] = voice.ID
	}
	return result
}
