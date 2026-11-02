package openai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/provider/schema"
	tool "github.com/mutablelogic/go-llm/toolkit/tool"
	jsonschema "github.com/mutablelogic/go-server/pkg/jsonschema"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

// waveTool is a tool without input, which waves
type waveTool struct {
	tool.Base
}

func (waveTool) Name() string                                      { return "wave" }
func (waveTool) Description() string                               { return "Wave" }
func (waveTool) InputSchema() *jsonschema.Schema                   { return nil }
func (waveTool) Run(context.Context, json.RawMessage) (any, error) { return "waved", nil }

// toolLoopServer is a stub server, whose model reasons and calls a tool, and
// then replies to its result, and which records the last request's messages
type toolLoopServer struct {
	messages []map[string]any
}

func (s *toolLoopServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	switch r.URL.Path {
	case "/v1/models":
		w.Write([]byte(`{"object":"list","data":[{"id":"deepseek-reasoner"}]}`))
	case "/v1/chat/completions":
		var req struct {
			Messages []map[string]any `json:"messages"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		s.messages = req.Messages
		if last := req.Messages[len(req.Messages)-1]; last["role"] == "tool" {
			w.Write([]byte(`{"id":"2","choices":[{"index":0,"message":{"role":"assistant","reasoning_content":"It worked.","content":"I waved"},"finish_reason":"stop"}],"usage":{"prompt_tokens":40,"completion_tokens":6,"completion_tokens_details":{"reasoning_tokens":3}}}`))
		} else {
			w.Write([]byte(`{"id":"1","choices":[{"index":0,"message":{"role":"assistant","reasoning_content":"I should wave.","content":null,"tool_calls":[{"id":"call_1","type":"function","function":{"name":"wave","arguments":"{}"}}]},"finish_reason":"tool_calls"}],"usage":{"prompt_tokens":20,"completion_tokens":8,"completion_tokens_details":{"reasoning_tokens":4}}}`))
		}
	default:
		http.NotFound(w, r)
	}
}

func TestToolLoop(t *testing.T) {
	ctx := context.Background()

	// loop returns the messages of the second request of a tool loop, with
	// the reply to the first, and the tool's result, for a client for the
	// host
	loop := func(t *testing.T, host string) []map[string]any {
		t.Helper()
		stub := new(toolLoopServer)
		server := httptest.NewServer(stub)
		t.Cleanup(server.Close)
		c, err := New(server.URL, "")
		require.NoError(t, err)
		c.url = &url.URL{Scheme: "https", Host: host, Path: "/v1"}
		model, err := c.GetModel(ctx, "deepseek-reasoner")
		require.NoError(t, err)

		user, err := schema.NewMessage("user", "Wave")
		require.NoError(t, err)
		tools := []llm.Tool{waveTool{}}
		reply, _, err := c.WithoutSession(ctx, model, []llm.Message{user}, tools, nil)
		require.NoError(t, err)
		require.Len(t, reply.ToolCalls(), 1)
		result := &schema.Message{Sender: "tool", Content: []schema.ContentBlock{schema.NewToolResult("call_1", "wave", "waved")}}
		reply, _, err = c.WithoutSession(ctx, model, []llm.Message{user, reply, result}, tools, nil)
		require.NoError(t, err)
		assert.Equal(t, "I waved", reply.Text())
		require.Len(t, stub.messages, 3)
		return stub.messages
	}

	t.Run("DeepSeek", func(t *testing.T) {
		// DeepSeek is sent the reasoning of each reply, with tools
		messages := loop(t, deepSeekHost)
		assert.Equal(t, "assistant", messages[1]["role"])
		assert.Equal(t, "I should wave.", messages[1]["reasoning_content"])
		assert.NotContains(t, messages[0], "reasoning_content")
	})

	t.Run("Other", func(t *testing.T) {
		// Other servers aren't
		messages := loop(t, "localhost")
		assert.NotContains(t, messages[1], "reasoning_content")
	})
}
