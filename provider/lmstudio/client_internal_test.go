package lmstudio

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	providerschema "github.com/mutablelogic/go-llm/provider/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

///////////////////////////////////////////////////////////////////////////////
// A FAKE SERVER

// fake is an LM Studio server, with its models, which records what it was
// asked
type fake struct {
	mu       sync.Mutex
	models   []map[string]any
	asked    []string
	unloaded []string
	chat     map[string]any
}

func newFake(t *testing.T) (*fake, *Client) {
	f := &fake{models: []map[string]any{
		{
			"type": "llm", "key": "google/gemma-4-12b-qat", "display_name": "Gemma 4 12B QAT", "publisher": "google",
			"size_bytes": 7151067268, "loaded_instances": []any{}, "max_context_length": 262144,
			"capabilities": map[string]any{"vision": true, "trained_for_tool_use": true, "reasoning": map[string]any{"allowed_options": []string{"off", "on"}, "default": "on"}},
		},
		{"type": "embedding", "key": "text-embedding-nomic-embed-text-v1.5", "display_name": "Nomic Embed Text v1.5", "loaded_instances": []any{}, "max_context_length": 2048},
		{
			"type": "llm", "key": "qwen/qwen3-4b", "display_name": "Qwen3 4B", "max_context_length": 32768,
			"loaded_instances": []any{map[string]any{"id": "qwen/qwen3-4b", "config": map[string]any{"context_length": 4096}}},
			"capabilities":     map[string]any{"vision": false, "trained_for_tool_use": false},
		},
	}}
	server := httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(server.Close)
	c, err := New(server.URL+"/v1", "")
	require.NoError(t, err)
	return f, c
}

func (f *fake) serve(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.asked = append(f.asked, r.Method+" "+r.URL.Path)
	var body map[string]any
	_ = json.NewDecoder(r.Body).Decode(&body)
	w.Header().Set("Content-Type", "application/json")
	reply := func(v any) { _ = json.NewEncoder(w).Encode(v) }
	switch r.Method + " " + r.URL.Path {
	case "GET /api/v1/models":
		reply(map[string]any{"models": f.models})
	case "POST /api/v1/models/download":
		reply(map[string]any{"job_id": "job-1", "status": "downloading", "total_size_bytes": 100})
	case "GET /api/v1/models/download/status/job-1":
		f.models = append(f.models, map[string]any{"type": "llm", "key": "new/model", "loaded_instances": []any{}})
		reply(map[string]any{"job_id": "job-1", "status": "completed", "total_size_bytes": 100, "downloaded_bytes": 100})
	case "POST /api/v1/models/load":
		for _, model := range f.models {
			if model["key"] == body["model"] {
				model["loaded_instances"] = []any{map[string]any{"id": body["model"], "config": map[string]any{"context_length": 8192}}}
			}
		}
		reply(map[string]any{"type": "llm", "instance_id": body["model"], "load_time_seconds": 1.5, "status": "loaded"})
	case "POST /api/v1/models/unload":
		f.unloaded = append(f.unloaded, body["instance_id"].(string))
		reply(map[string]any{"instance_id": body["instance_id"]})
	case "POST /v1/chat/completions":
		f.chat = body
		reply(map[string]any{
			"choices": []any{map[string]any{"index": 0, "message": map[string]any{"role": "assistant", "content": "Hello"}, "finish_reason": "stop"}},
			"usage":   map[string]any{"prompt_tokens": 5, "completion_tokens": 1, "total_tokens": 6},
		})
	default:
		w.WriteHeader(http.StatusNotFound)
		reply(map[string]any{"error": "not found"})
	}
}

///////////////////////////////////////////////////////////////////////////////
// TESTS

func Test_RootURL(t *testing.T) {
	for in, want := range map[string]string{
		"":                             "http://localhost:1234",
		"localhost:1235":               "http://localhost:1235",
		"http://localhost:1235/v1":     "http://localhost:1235",
		"http://localhost:1235/v1/":    "http://localhost:1235",
		"http://localhost:1235/api/v1": "http://localhost:1235",
		"https://studio.example/lm/v1": "https://studio.example/lm",
	} {
		u, err := rootURL(in)
		require.NoError(t, err, in)
		assert.Equal(t, want, u.String(), in)
	}
	_, err := rootURL("http://")
	assert.ErrorIs(t, err, llm.ErrBadParameter)

	c, err := New("localhost:1235", "")
	require.NoError(t, err)
	assert.Equal(t, "http://localhost:1235/v1", c.URL().String(), "its URL is its OpenAI-compatible API's")
	assert.Equal(t, llm.LMStudio, c.Name())
}

func Test_ListModels(t *testing.T) {
	_, c := newFake(t)
	models, err := c.ListModels(context.Background())
	require.NoError(t, err)
	require.Len(t, models, 3)

	// A language model, with what it can do, and the most context it can
	// have, as it isn't loaded
	gemma := models[0]
	assert.Equal(t, "google/gemma-4-12b-qat", gemma.Name())
	assert.Equal(t, "Gemma 4 12B QAT", gemma.Description())
	assert.Equal(t, uint(262144), types.Value(gemma.InputTokenLimit()))
	can := gemma.Capabilities()
	assert.Equal(t, "chat, tools, thinking, image_input", can.String())
	assert.True(t, types.Value(can.Temperature))
	assert.Contains(t, gemma.Meta(), "publisher")
	assert.NotContains(t, gemma.Meta(), "key")

	// An embedding model
	assert.Equal(t, "embedding", models[1].Capabilities().String())

	// A model which is loaded has the context it was loaded with
	qwen := models[2]
	assert.Equal(t, uint(4096), types.Value(qwen.InputTokenLimit()))
	assert.Equal(t, "chat", qwen.Capabilities().String())

	_, err = c.GetModel(context.Background(), "missing")
	assert.ErrorIs(t, err, llm.ErrNotFound)
	require.NoError(t, c.Ping(context.Background()))
}

func Test_LoadModel(t *testing.T) {
	downloadPoll = 10 * time.Millisecond
	f, c := newFake(t)
	ctx := context.Background()
	var statuses []string
	progress := func(status string, percent float64) { statuses = append(statuses, status) }

	// A model which is stored is loaded, without downloading it
	model, err := c.LoadModel(ctx, "google/gemma-4-12b-qat", progress)
	require.NoError(t, err)
	assert.True(t, model.(*Model).Loaded())
	assert.Equal(t, uint(8192), types.Value(model.InputTokenLimit()), "with the context it was loaded with")
	assert.Equal(t, []string{"loading", "loaded"}, statuses)
	assert.NotContains(t, f.asked, "POST /api/v1/models/download")

	// One which is loaded isn't loaded again
	f.asked, statuses = nil, nil
	_, err = c.LoadModel(ctx, "qwen/qwen3-4b", progress)
	require.NoError(t, err)
	assert.NotContains(t, f.asked, "POST /api/v1/models/load")
	assert.Empty(t, statuses)

	// One which isn't stored is downloaded, and then loaded
	f.asked, statuses = nil, nil
	model, err = c.LoadModel(ctx, "new/model", progress)
	require.NoError(t, err)
	assert.Equal(t, "new/model", model.Name())
	assert.Equal(t, []string{"downloading", "completed", "loading", "loaded"}, statuses)
	assert.Contains(t, f.asked, "GET /api/v1/models/download/status/job-1")

	_, err = c.LoadModel(ctx, " ", nil)
	assert.ErrorIs(t, err, llm.ErrBadParameter)
}

func Test_UnloadModel(t *testing.T) {
	f, c := newFake(t)
	ctx := context.Background()
	model, err := c.GetModel(ctx, "qwen/qwen3-4b")
	require.NoError(t, err)
	require.NoError(t, c.UnloadModel(ctx, model))
	assert.Equal(t, []string{"qwen/qwen3-4b"}, f.unloaded, "each instance which is loaded")

	// One which isn't loaded has nothing to unload
	f.unloaded = nil
	model, err = c.GetModel(ctx, "google/gemma-4-12b-qat")
	require.NoError(t, err)
	require.NoError(t, c.UnloadModel(ctx, model))
	assert.Empty(t, f.unloaded)

	assert.ErrorIs(t, c.DeleteModel(ctx, model), llm.ErrNotImplemented)
	assert.ErrorIs(t, c.UnloadModel(ctx, nil), llm.ErrBadParameter)
}

func Test_WithoutSession(t *testing.T) {
	f, c := newFake(t)
	ctx := context.Background()
	model, err := c.GetModel(ctx, "google/gemma-4-12b-qat")
	require.NoError(t, err)
	message := &providerschema.Message{MessageMeta: providerschema.MessageMeta{Sender: "user", Content: []providerschema.ContentBlock{{Text: types.Ptr("Hi")}}}}
	reply, usage, err := c.WithoutSession(ctx, model, []llm.Message{message}, nil, nil)
	require.NoError(t, err)
	assert.Equal(t, "Hello", strings.TrimSpace(reply.Text()))
	assert.Equal(t, uint(5), usage.InputTokens)
	assert.Equal(t, "google/gemma-4-12b-qat", f.chat["model"], "with the OpenAI-compatible API")

	// A model of another provider isn't asked
	_, other := newFake(t)
	_, _, err = other.WithoutSession(ctx, model, []llm.Message{message}, nil, nil)
	assert.ErrorIs(t, err, llm.ErrBadParameter)
}
