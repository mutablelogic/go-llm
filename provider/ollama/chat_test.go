package ollama_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	eliza "github.com/mutablelogic/go-llm/provider/eliza"
	ollama "github.com/mutablelogic/go-llm/provider/ollama"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

// chatServer is a stub Ollama server, which replies to chats, and records
// the last chat request
type chatServer struct {
	sync.Mutex
	request map[string]any
}

func (s *chatServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == http.MethodPost && r.URL.Path == "/api/show":
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"details":{"family":"llama"},"model_info":{"llama.context_length":131072}}`))
	case r.Method == http.MethodPost && r.URL.Path == "/api/chat":
		var req map[string]any
		json.NewDecoder(r.Body).Decode(&req)
		s.Lock()
		s.request = req
		s.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if req["model"] == "truncated:latest" {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"error":"no user query found in messages"}`))
			return
		}
		if req["model"] == "missing:latest" {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"error":"model \"missing:latest\" not found, try pulling it first"}`))
			return
		}
		if req["stream"] == true {
			// A streamed reply is chunks of thinking and text, and a tool
			// call, and then a chunk which is done, or which fails or ends
			// early, for some models
			w.Header().Set("Content-Type", "application/x-ndjson")
			chunks := []string{
				`{"model":"llama3.2:latest","message":{"role":"assistant","content":"","thinking":"Hmm, "},"done":false}`,
				`{"model":"llama3.2:latest","message":{"role":"assistant","content":"","thinking":"a greeting."},"done":false}`,
				`{"model":"llama3.2:latest","message":{"role":"assistant","content":"Hello"},"done":false}`,
				`{"model":"llama3.2:latest","message":{"role":"assistant","content":" there"},"done":false}`,
				`{"model":"llama3.2:latest","message":{"role":"assistant","content":"","tool_calls":[{"function":{"name":"wave","arguments":{"hand":"left"}}}]},"done":false}`,
			}
			switch req["model"] {
			case "failing:latest":
				chunks = append(chunks[:3], `{"error":"model runner has unexpectedly stopped"}`)
			case "ending:latest":
				chunks = chunks[:3]
			default:
				chunks = append(chunks, `{"model":"llama3.2:latest","created_at":"2026-10-11T08:52:19.385406455Z","message":{"role":"assistant","content":""},"done":true,"done_reason":"stop","prompt_eval_count":26,"eval_count":5}`)
			}
			for _, chunk := range chunks {
				w.Write([]byte(chunk + "\n"))
			}
			return
		}
		w.Write([]byte(`{
			"model": "llama3.2:latest",
			"created_at": "2026-10-11T08:52:19.385406455Z",
			"message": {"role": "assistant", "content": "Hello there"},
			"done": true,
			"done_reason": "stop",
			"prompt_eval_count": 26,
			"eval_count": 3
		}`))
	default:
		http.NotFound(w, r)
	}
}

// message is a message to send
type message struct {
	role, text string
}

func (m message) Role() string                  { return m.role }
func (m message) Text() string                  { return m.text }
func (m message) Resources() []llm.Resource     { return nil }
func (m message) ToolCalls() []llm.ToolCall     { return nil }
func (m message) ToolResults() []llm.ToolResult { return nil }

func newChatClient(t *testing.T) (*ollama.Client, *chatServer) {
	t.Helper()
	stub := new(chatServer)
	server := httptest.NewServer(stub)
	t.Cleanup(server.Close)
	c, err := ollama.New(server.URL)
	require.NoError(t, err)
	return c, stub
}

func Test_ImplementsGenerator(t *testing.T) {
	assert.Implements(t, (*llm.Generator)(nil), &ollama.Client{})
}

func Test_WithoutSession(t *testing.T) {
	c, stub := newChatClient(t)
	ctx := context.Background()
	messages := []llm.Message{
		message{role: "system", text: "Be brief"},
		message{role: "user", text: "Hello"},
	}

	t.Run("Reply", func(t *testing.T) {
		model, err := c.GetModel(ctx, "llama3.2:latest")
		require.NoError(t, err)

		reply, usage, err := c.WithoutSession(ctx, model, messages, nil, nil)
		require.NoError(t, err)
		assert.Equal(t, "assistant", reply.Role())
		assert.Equal(t, "Hello there", reply.Text())
		assert.Equal(t, llm.Usage{InputTokens: 26, OutputTokens: 3}, usage)

		// Without a stream function, the request isn't streamed, and has
		// the messages
		stub.Lock()
		defer stub.Unlock()
		assert.Equal(t, "llama3.2:latest", stub.request["model"])
		assert.Equal(t, false, stub.request["stream"])
		assert.Equal(t, []any{
			map[string]any{"role": "system", "content": "Be brief"},
			map[string]any{"role": "user", "content": "Hello"},
		}, stub.request["messages"])
	})

	t.Run("Stream", func(t *testing.T) {
		// The stream function is called with each piece of thinking and
		// text, and the reply has them all, with the tool call, the reason
		// the model stopped, and the tokens used
		model, err := c.GetModel(ctx, "llama3.2:latest")
		require.NoError(t, err)
		var streamed []string
		reply, usage, err := c.WithoutSession(ctx, model, messages, nil, func(what llm.StreamType, text string) {
			streamed = append(streamed, string(what)+":"+text)
		})
		require.NoError(t, err)
		assert.Equal(t, []string{"thinking:Hmm, ", "thinking:a greeting.", "text:Hello", "text: there"}, streamed)
		assert.Equal(t, "assistant", reply.Role())
		assert.Equal(t, "Hello there", reply.Text())
		require.Len(t, reply.ToolCalls(), 1)
		assert.Equal(t, "wave", reply.ToolCalls()[0].Name)
		assert.Equal(t, llm.Usage{InputTokens: 26, OutputTokens: 5}, usage)
		stub.Lock()
		assert.Equal(t, true, stub.request["stream"])
		stub.Unlock()
	})

	t.Run("StreamError", func(t *testing.T) {
		// An error in the stream, or a stream which ends before it's done,
		// fails the reply
		for _, name := range []string{"failing:latest", "ending:latest"} {
			model, err := c.GetModel(ctx, name)
			require.NoError(t, err)
			_, _, err = c.WithoutSession(ctx, model, messages, nil, func(llm.StreamType, string) {})
			assert.Error(t, err, name)
		}
	})

	t.Run("Truncated", func(t *testing.T) {
		// Ollama fails when the user's message is removed from a prompt
		// which is more than its context window
		model, err := c.GetModel(ctx, "truncated:latest")
		require.NoError(t, err)
		_, _, err = c.WithoutSession(ctx, model, messages, nil, nil)
		assert.ErrorIs(t, err, llm.ErrBadParameter)
		assert.ErrorContains(t, err, "context window")
	})

	t.Run("ModelNotFound", func(t *testing.T) {
		model, err := c.GetModel(ctx, "missing:latest")
		require.NoError(t, err)
		_, _, err = c.WithoutSession(ctx, model, messages, nil, nil)
		assert.ErrorContains(t, err, "not found")
	})

	t.Run("NotOllama", func(t *testing.T) {
		other, err := eliza.New()
		require.NoError(t, err)
		model, err := other.GetModel(ctx, eliza.ModelName)
		require.NoError(t, err)
		_, _, err = c.WithoutSession(ctx, model, messages, nil, nil)
		assert.ErrorIs(t, err, llm.ErrBadParameter)
		_, _, err = c.WithoutSession(ctx, nil, messages, nil, nil)
		assert.ErrorIs(t, err, llm.ErrBadParameter)
	})

	t.Run("NoMessages", func(t *testing.T) {
		model, err := c.GetModel(ctx, "llama3.2:latest")
		require.NoError(t, err)
		_, _, err = c.WithoutSession(ctx, model, nil, nil, nil)
		assert.ErrorIs(t, err, llm.ErrBadParameter)
	})

	t.Run("BadMessage", func(t *testing.T) {
		model, err := c.GetModel(ctx, "llama3.2:latest")
		require.NoError(t, err)
		_, _, err = c.WithoutSession(ctx, model, []llm.Message{message{role: "robot"}}, nil, nil)
		assert.ErrorIs(t, err, llm.ErrBadParameter)
	})
}
