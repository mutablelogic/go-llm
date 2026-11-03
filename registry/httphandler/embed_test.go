package httphandler_test

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/registry/schema"
	test "github.com/mutablelogic/go-llm/registry/test"
	types "github.com/mutablelogic/go-server/pkg/types"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

// embedInput is an input to embed, as the stub Ollama server receives it
type embedInput struct {
	Text  string `json:"text"`
	Image []byte `json:"image"`
}

// stubEmbedder is a stub Ollama server, with a model which embeds text and
// images, and one which doesn't embed, which keeps the inputs it was asked
// to embed
type stubEmbedder struct {
	sync.Mutex
	inputs     []json.RawMessage
	dimensions uint
}

func (s *stubEmbedder) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name       string            `json:"name"`
		Model      string            `json:"model"`
		Input      []json.RawMessage `json:"input"`
		Dimensions uint              `json:"dimensions"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	w.Header().Set("Content-Type", "application/json")
	switch r.URL.Path {
	case "/api/version":
		w.Write([]byte(`{"version":"0.0.1"}`))
	case "/api/tags":
		w.Write([]byte(`{"models":[{"name":"embed:latest","details":{}},{"name":"chat:latest","details":{}}]}`))
	case "/api/show":
		if req.Name == "embed:latest" || req.Model == "embed:latest" {
			w.Write([]byte(`{"details":{},"capabilities":["embedding","vision"]}`))
		} else {
			w.Write([]byte(`{"details":{},"capabilities":["completion"]}`))
		}
	case "/api/embed":
		// A message which is a status, such as "status 500", is an error
		// with the status
		var text string
		if len(req.Input) > 0 && json.Unmarshal(req.Input[0], &text) == nil {
			if code, ok := strings.CutPrefix(text, "status "); ok {
				status, _ := strconv.Atoi(code)
				w.WriteHeader(status)
				w.Write([]byte(`{"error":"upstream error"}`))
				return
			}
		}
		s.Lock()
		s.inputs, s.dimensions = req.Input, req.Dimensions
		s.Unlock()
		dimensions := max(req.Dimensions, 3)
		embeddings := make([][]float32, len(req.Input))
		for i := range embeddings {
			embeddings[i] = make([]float32, dimensions)
			embeddings[i][0] = float32(i + 1)
		}
		json.NewEncoder(w).Encode(map[string]any{"model": req.Model, "embeddings": embeddings, "prompt_eval_count": 5})
	default:
		http.NotFound(w, r)
	}
}

// Input returns the input to embed with the index, and the dimensions, of
// the last request
func (s *stubEmbedder) Input(t *testing.T, i int) (embedInput, uint) {
	t.Helper()
	s.Lock()
	defer s.Unlock()
	require.Greater(t, len(s.inputs), i)
	var input embedInput
	if json.Unmarshal(s.inputs[i], &input.Text) != nil {
		require.NoError(t, json.Unmarshal(s.inputs[i], &input))
	}
	return input, s.dimensions
}

// newEmbedServer returns a server of a registry with a stub Ollama server,
// as home, which embeds, and eliza, which doesn't
func newEmbedServer(t *testing.T) (*test.Server, *stubEmbedder) {
	t.Helper()
	stub := new(stubEmbedder)
	ollama := httptest.NewServer(stub)
	t.Cleanup(ollama.Close)
	r := test.NewRegistry(t, nil)
	_, err := r.CreateProvider(t.Context(), schema.Provider{Name: "home", Type: llm.Ollama, URL: ollama.URL})
	require.NoError(t, err)
	_, err = r.CreateProvider(t.Context(), schema.Provider{Type: llm.Eliza})
	require.NoError(t, err)
	return test.NewServer(t, r), stub
}

// text returns a message to embed with the text
func text(value string) schema.EmbedMessage {
	return schema.EmbedMessage{Content: []schema.EmbedContent{{Text: types.Ptr(value)}}}
}

func Test_Embed(t *testing.T) {
	server, stub := newEmbedServer(t)
	png := []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR")

	t.Run("JSON", func(t *testing.T) {
		// A vector for each message, in order, with the model's canonical
		// name, and the tokens, and the options are the provider's
		var response schema.EmbedResponse
		require.Equal(t, http.StatusOK, do(t, server, http.MethodPost, "llm/embed", schema.EmbedRequest{
			Model:    "embed",
			Messages: []schema.EmbedMessage{text("A cat"), text("A dog")},
			Opt:      map[string]json.RawMessage{"dimensions": json.RawMessage(`4`)},
		}, &response))
		assert.Equal(t, "home:embed:latest", response.Model)
		if assert.Len(t, response.Vectors, 2) {
			assert.Equal(t, llm.Vector{1, 0, 0, 0}, response.Vectors[0])
			assert.Equal(t, llm.Vector{2, 0, 0, 0}, response.Vectors[1])
		}
		assert.Equal(t, uint(5), response.Usage.InputTokens)
		input, dimensions := stub.Input(t, 1)
		assert.Equal(t, "A dog", input.Text)
		assert.Equal(t, uint(4), dimensions)
	})

	t.Run("Attachment", func(t *testing.T) {
		// A message's image is embedded with its text
		var response schema.EmbedResponse
		require.Equal(t, http.StatusOK, do(t, server, http.MethodPost, "llm/embed", schema.EmbedRequest{
			Model: "home:embed:latest",
			Messages: []schema.EmbedMessage{{Content: []schema.EmbedContent{
				{Text: types.Ptr("A cat")},
				{Attachment: &schema.EmbedAttachment{Type: "image/png", Data: png}},
			}}},
		}, &response))
		assert.Len(t, response.Vectors, 1)
		input, _ := stub.Input(t, 0)
		assert.Equal(t, "A cat", input.Text)
		assert.Equal(t, png, input.Image)
	})

	// postForm posts a form with the fields, and the files, by name, and
	// returns the status, and the body
	postForm := func(fields map[string]string, files ...string) (int, []byte) {
		var body bytes.Buffer
		form := multipart.NewWriter(&body)
		for key, value := range fields {
			require.NoError(t, form.WriteField(key, value))
		}
		for _, name := range files {
			file, err := form.CreateFormFile("file", name)
			require.NoError(t, err)
			if strings.HasSuffix(name, ".txt") {
				file.Write([]byte("A cat"))
			} else {
				file.Write(png)
			}
		}
		require.NoError(t, form.Close())
		return server.DoWithContentType(t, http.MethodPost, "llm/embed", form.FormDataContentType(), body.Bytes())
	}

	t.Run("Form", func(t *testing.T) {
		// A form embeds one message, which is a file, whose type is
		// detected when it's generic, as text when it's text, with the
		// options as JSON
		status, data := postForm(map[string]string{"model": "embed", "opt": `{"dimensions":2}`}, "cat.txt")
		require.Equal(t, http.StatusOK, status, string(data))
		var response schema.EmbedResponse
		require.NoError(t, json.Unmarshal(data, &response))
		assert.Equal(t, "home:embed:latest", response.Model)
		input, dimensions := stub.Input(t, 0)
		assert.Equal(t, "A cat", input.Text)
		assert.Nil(t, input.Image)
		assert.Equal(t, uint(2), dimensions)

		// Or as an attachment otherwise
		status, data = postForm(map[string]string{"model": "embed"}, "cat")
		require.Equal(t, http.StatusOK, status, string(data))
		input, _ = stub.Input(t, 0)
		assert.Empty(t, input.Text)
		assert.Equal(t, png, input.Image)
	})

	t.Run("FormErrors", func(t *testing.T) {
		// A form with no file, or more than one, or options which aren't
		// JSON, isn't valid
		for _, test := range []struct {
			fields map[string]string
			files  []string
		}{
			{map[string]string{"model": "embed"}, nil},
			{map[string]string{"model": "embed"}, []string{"a.png", "b.png"}},
			{map[string]string{"model": "embed", "opt": "not json"}, []string{"cat.txt"}},
		} {
			status, _ := postForm(test.fields, test.files...)
			assert.Equal(t, http.StatusBadRequest, status, test)
		}
	})

	t.Run("Errors", func(t *testing.T) {
		post := func(req schema.EmbedRequest) int {
			return do(t, server, http.MethodPost, "llm/embed", req, nil)
		}

		// A request needs a model, and messages, each with text or an
		// attachment, with its type and data
		assert.Equal(t, http.StatusBadRequest, post(schema.EmbedRequest{Messages: []schema.EmbedMessage{text("A cat")}}))
		assert.Equal(t, http.StatusBadRequest, post(schema.EmbedRequest{Model: "embed"}))
		assert.Equal(t, http.StatusBadRequest, post(schema.EmbedRequest{Model: "embed", Messages: []schema.EmbedMessage{text(" ")}}))
		assert.Equal(t, http.StatusBadRequest, post(schema.EmbedRequest{Model: "embed", Messages: []schema.EmbedMessage{{Content: []schema.EmbedContent{
			{Attachment: &schema.EmbedAttachment{Type: "image/png"}},
		}}}}))

		// And a model which exists, and embeds, whose provider embeds
		assert.Equal(t, http.StatusNotFound, post(schema.EmbedRequest{Model: "missing", Messages: []schema.EmbedMessage{text("A cat")}}))
		assert.Equal(t, http.StatusNotImplemented, post(schema.EmbedRequest{Model: "home:chat:latest", Messages: []schema.EmbedMessage{text("A cat")}}))
		assert.Equal(t, http.StatusNotImplemented, post(schema.EmbedRequest{Model: "eliza", Messages: []schema.EmbedMessage{text("A cat")}}))

		// And a provider which rejects the request is a request which isn't
		// valid, but a provider which fails otherwise, such as its own
		// authentication, is unavailable, rather than the endpoint's status
		for status, want := range map[int]int{
			http.StatusBadRequest:          http.StatusBadRequest,
			http.StatusUnprocessableEntity: http.StatusBadRequest,
			http.StatusUnauthorized:        http.StatusServiceUnavailable,
			http.StatusForbidden:           http.StatusServiceUnavailable,
			http.StatusTooManyRequests:     http.StatusServiceUnavailable,
			http.StatusInternalServerError: http.StatusServiceUnavailable,
		} {
			assert.Equal(t, want, post(schema.EmbedRequest{Model: "embed", Messages: []schema.EmbedMessage{text("status " + strconv.Itoa(status))}}), status)
		}

		// And options the provider takes
		assert.Equal(t, http.StatusBadRequest, post(schema.EmbedRequest{Model: "embed", Messages: []schema.EmbedMessage{text("A cat")}, Opt: map[string]json.RawMessage{"dimensions": json.RawMessage(`"many"`)}}))
	})
}
