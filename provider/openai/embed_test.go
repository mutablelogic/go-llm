package openai_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	eliza "github.com/mutablelogic/go-llm/provider/eliza"
	openai "github.com/mutablelogic/go-llm/provider/openai"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

// embedServer is a stub OpenAI-compatible server, which embeds each text as
// a vector of its length and index, in reverse order, and records the last
// request
type embedServer struct {
	sync.Mutex
	request map[string]any
}

func (s *embedServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/v1/models":
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"object":"list","data":[{"id":"text-embedding-3-small"},{"id":"short"},{"id":"repeated"},{"id":"erroring"},{"id":"missing"}]}`))
	case r.Method == http.MethodPost && r.URL.Path == "/v1/embeddings":
		var req struct {
			Model string   `json:"model"`
			Input []string `json:"input"`
		}
		var raw map[string]any
		json.NewDecoder(r.Body).Decode(&raw)
		data, _ := json.Marshal(raw)
		json.Unmarshal(data, &req)
		s.Lock()
		s.request = raw
		s.Unlock()
		w.Header().Set("Content-Type", "application/json")
		switch req.Model {
		case "missing":
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"error":{"message":"The model does not exist","type":"invalid_request_error"}}`))
			return
		case "erroring":
			w.Write([]byte(`{"error":{"message":"input is too long","type":"invalid_request_error"}}`))
			return
		}
		type item struct {
			Index     int       `json:"index"`
			Embedding []float32 `json:"embedding"`
		}
		var items []item
		for i := len(req.Input) - 1; i >= 0; i-- {
			index := i
			if req.Model == "repeated" {
				index = 0
			}
			items = append(items, item{index, []float32{float32(len(req.Input[i])), float32(i)}})
		}
		if req.Model == "short" {
			items = items[1:]
		}
		json.NewEncoder(w).Encode(map[string]any{"object": "list", "model": req.Model, "data": items, "usage": map[string]any{"prompt_tokens": 9, "total_tokens": 9}})
	default:
		http.NotFound(w, r)
	}
}

// mediaMessage is a message with an image
type mediaMessage struct{}

func (mediaMessage) Role() string                  { return "user" }
func (mediaMessage) Text() string                  { return "A cat" }
func (mediaMessage) Resources() []llm.Resource     { return []llm.Resource{image{}} }
func (mediaMessage) ToolCalls() []llm.ToolCall     { return nil }
func (mediaMessage) ToolResults() []llm.ToolResult { return nil }

type image struct{}

func (image) URI() string         { return "" }
func (image) Name() string        { return "" }
func (image) Description() string { return "" }
func (image) Type() string        { return "image/png" }
func (image) Read(context.Context) (llm.ReadFunc, error) {
	return func(_ context.Context, fn func(io.Reader) error) error {
		return fn(strings.NewReader("png"))
	}, nil
}

func Test_ImplementsEmbedder(t *testing.T) {
	assert.Implements(t, (*llm.Embedder)(nil), &openai.Client{})
}

func Test_Embed(t *testing.T) {
	stub := new(embedServer)
	server := httptest.NewServer(stub)
	t.Cleanup(server.Close)
	c, err := openai.New(server.URL, apiKey)
	require.NoError(t, err)
	ctx := context.Background()
	model := func(t *testing.T, name string) llm.Model {
		t.Helper()
		model, err := c.GetModel(ctx, name)
		require.NoError(t, err)
		return model
	}
	messages := []llm.Message{message{role: "user", text: "Hello"}, message{role: "user", text: "Goodbye, world"}}

	t.Run("Vectors", func(t *testing.T) {
		// A vector for each message, in order by index, whatever the order
		// they're in, in one request, as numbers, with the tokens
		vectors, usage, err := c.Embed(ctx, model(t, "text-embedding-3-small"), messages)
		require.NoError(t, err)
		assert.Equal(t, []llm.Vector{{5, 0}, {14, 1}}, vectors)
		assert.Equal(t, llm.Usage{InputTokens: 9}, usage)
		stub.Lock()
		defer stub.Unlock()
		assert.Equal(t, "text-embedding-3-small", stub.request["model"])
		assert.Equal(t, []any{"Hello", "Goodbye, world"}, stub.request["input"])
		assert.Equal(t, "float", stub.request["encoding_format"])
		assert.NotContains(t, stub.request, "dimensions")
	})

	t.Run("Options", func(t *testing.T) {
		// The dimensions and user are the API's, and the task type and title,
		// which it hasn't, are ignored
		_, _, err := c.Embed(ctx, model(t, "text-embedding-3-small"), messages, llm.WithDimensions(256), llm.WithUser("user-1"), llm.WithTaskType("retrieval_query"), llm.WithTitle("Notes"))
		require.NoError(t, err)
		stub.Lock()
		defer stub.Unlock()
		assert.Equal(t, float64(256), stub.request["dimensions"])
		assert.Equal(t, "user-1", stub.request["user"])
		assert.NotContains(t, stub.request, "task_type")

		// A generation option isn't supported
		_, _, err = c.Embed(ctx, model(t, "text-embedding-3-small"), messages, llm.WithTemperature(0.5))
		assert.ErrorIs(t, err, llm.ErrBadParameter)
	})

	t.Run("Errors", func(t *testing.T) {
		// Messages are needed, each with text and without media, and a model
		// of this client
		embedding := model(t, "text-embedding-3-small")
		_, _, err := c.Embed(ctx, embedding, nil)
		assert.ErrorIs(t, err, llm.ErrBadParameter)
		_, _, err = c.Embed(ctx, embedding, []llm.Message{message{role: "user", text: ""}})
		assert.ErrorIs(t, err, llm.ErrBadParameter)
		_, _, err = c.Embed(ctx, embedding, []llm.Message{mediaMessage{}})
		assert.ErrorIs(t, err, llm.ErrBadParameter)
		other, err := eliza.New()
		require.NoError(t, err)
		elizaModel, err := other.GetModel(ctx, eliza.ModelName)
		require.NoError(t, err)
		_, _, err = c.Embed(ctx, elizaModel, messages)
		assert.ErrorIs(t, err, llm.ErrBadParameter)

		// A model which doesn't exist, an error in the reply, and a reply
		// without a vector for each message, or with one twice, are errors
		_, _, err = c.Embed(ctx, model(t, "missing"), messages)
		assert.Error(t, err)
		_, _, err = c.Embed(ctx, model(t, "erroring"), messages)
		assert.ErrorContains(t, err, "input is too long")
		_, _, err = c.Embed(ctx, model(t, "short"), messages)
		assert.ErrorIs(t, err, llm.ErrInternalServerError)
		_, _, err = c.Embed(ctx, model(t, "repeated"), messages)
		assert.ErrorIs(t, err, llm.ErrInternalServerError)
	})
}
