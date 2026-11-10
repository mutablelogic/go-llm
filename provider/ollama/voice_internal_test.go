package ollama

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

func Test_ListVoices(t *testing.T) {
	// The voices which Ollama shows for a speech model
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path != "/api/show":
			w.WriteHeader(http.StatusNotFound)
		case strings.Contains(string(body), "phonon"):
			_, _ = io.WriteString(w, `{"capabilities":["speech"],"voices":["Alfie","Bryony",""]}`)
		default:
			_, _ = io.WriteString(w, `{"capabilities":["completion"]}`)
		}
	}))
	t.Cleanup(server.Close)
	c, err := New(server.URL + "/api")
	require.NoError(t, err)
	ctx := context.Background()

	voices, err := c.ListVoices(ctx, &Model{provider: c, data: apiModel{Name: "phonon:latest"}})
	require.NoError(t, err)
	assert.Equal(t, &llm.VoiceList{Body: []llm.Voice{{ID: "Alfie", Title: "Alfie"}, {ID: "Bryony", Title: "Bryony"}}, Count: 2}, voices)

	// A model which doesn't speak has none
	_, err = c.ListVoices(ctx, &Model{provider: c, data: apiModel{Name: "llama3.2:latest"}})
	assert.ErrorIs(t, err, llm.ErrNotImplemented)
}
