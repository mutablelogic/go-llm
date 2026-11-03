package httpclient_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	httpclient "github.com/mutablelogic/go-llm/registry/httpclient"
	schema "github.com/mutablelogic/go-llm/registry/schema"
	test "github.com/mutablelogic/go-llm/registry/test"
	types "github.com/mutablelogic/go-server/pkg/types"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

// newEmbedClient returns a client of a test server of a registry with a stub
// Ollama server, as home, with a model which embeds text
func newEmbedClient(t *testing.T) *httpclient.Client {
	t.Helper()
	ollama := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Input []json.RawMessage `json:"input"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/version":
			w.Write([]byte(`{"version":"0.0.1"}`))
		case "/api/tags":
			w.Write([]byte(`{"models":[{"name":"embed:latest","details":{}}]}`))
		case "/api/show":
			w.Write([]byte(`{"details":{},"capabilities":["embedding"]}`))
		case "/api/embed":
			embeddings := make([][]float32, len(req.Input))
			for i := range embeddings {
				embeddings[i] = []float32{float32(i + 1), 0}
			}
			json.NewEncoder(w).Encode(map[string]any{"embeddings": embeddings, "prompt_eval_count": 3})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(ollama.Close)
	r := test.NewRegistry(t, nil)
	_, err := r.CreateProvider(t.Context(), schema.Provider{Name: "home", Type: llm.Ollama, URL: ollama.URL})
	require.NoError(t, err)
	client, err := httpclient.New(test.NewServer(t, r).URL + test.Prefix)
	require.NoError(t, err)
	return client
}

func Test_ClientEmbed(t *testing.T) {
	ctx := t.Context()
	client := newEmbedClient(t)
	message := func(text string) schema.EmbedMessage {
		return schema.EmbedMessage{Content: []schema.EmbedContent{{Text: types.Ptr(text)}}}
	}

	t.Run("Embed", func(t *testing.T) {
		// A vector for each message, with the model's canonical name
		response, err := client.Embed(ctx, schema.EmbedRequest{Model: "embed", Messages: []schema.EmbedMessage{message("A cat"), message("A dog")}})
		require.NoError(t, err)
		assert.Equal(t, "home:embed:latest", response.Model)
		assert.Equal(t, []llm.Vector{{1, 0}, {2, 0}}, response.Vectors)
		assert.Equal(t, uint(3), response.Usage.InputTokens)
	})

	t.Run("Errors", func(t *testing.T) {
		// A request needs a model, and messages, before it's sent, and a
		// model which exists
		_, err := client.Embed(ctx, schema.EmbedRequest{Messages: []schema.EmbedMessage{message("A cat")}})
		assert.ErrorIs(t, err, llm.ErrBadParameter)
		_, err = client.Embed(ctx, schema.EmbedRequest{Model: "embed"})
		assert.ErrorIs(t, err, llm.ErrBadParameter)
		_, err = client.Embed(ctx, schema.EmbedRequest{Model: "missing", Messages: []schema.EmbedMessage{message("A cat")}})
		assert.Equal(t, http.StatusNotFound, status(t, err))
	})
}
