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

// newRecognizeClient returns a client of a test server of a registry with a
// stub Ollama server, as home, with a model which recognizes text
func newRecognizeClient(t *testing.T) *httpclient.Client {
	t.Helper()
	ollama := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/version":
			w.Write([]byte(`{"version":"0.0.1"}`))
		case "/api/tags":
			w.Write([]byte(`{"models":[{"name":"deepseek-ocr:latest","details":{}}]}`))
		case "/api/show":
			w.Write([]byte(`{"details":{},"capabilities":["completion","vision"]}`))
		case "/api/chat":
			w.Write([]byte(`{"message":{"role":"assistant","content":"# Hello"},"done":true,"prompt_eval_count":900,"eval_count":3}`))
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

func Test_ClientRecognize(t *testing.T) {
	ctx := t.Context()
	client := newRecognizeClient(t)
	png := []byte("\x89PNG\r\n\x1a\n")

	t.Run("Recognize", func(t *testing.T) {
		response, err := client.Recognize(ctx, schema.RecognizeRequest{Model: "deepseek-ocr", Type: "image/png", Data: png})
		require.NoError(t, err)
		assert.Equal(t, "home:deepseek-ocr:latest", response.Model)
		assert.Equal(t, "# Hello", response.Message.Text())
		assert.Equal(t, uint(1), response.Usage.Pages)
	})

	t.Run("Errors", func(t *testing.T) {
		// A request needs a model, a type, and data, before it's sent, and
		// a model which exists
		for _, req := range []schema.RecognizeRequest{
			{Type: "image/png", Data: png},
			{Model: "deepseek-ocr", Data: png},
			{Model: "deepseek-ocr", Type: "image/png"},
		} {
			_, err := client.Recognize(ctx, req)
			assert.ErrorIs(t, err, llm.ErrBadParameter)
		}
		_, err := client.Recognize(ctx, schema.RecognizeRequest{Model: "missing", Type: "image/png", Data: png})
		assert.Error(t, err)
	})
}
