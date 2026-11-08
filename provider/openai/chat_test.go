package openai_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	eliza "github.com/mutablelogic/go-llm/provider/eliza"
	openai "github.com/mutablelogic/go-llm/provider/openai"
	schema "github.com/mutablelogic/go-llm/provider/schema"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

// chatServer is a stub OpenAI-compatible server, which replies to chats, and
// records the last chat request
type chatServer struct {
	sync.Mutex
	request map[string]any
}

func (s *chatServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/v1/models":
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"object":"list","data":[{"id":"deepseek-chat"},{"id":"missing"},{"id":"failing"},{"id":"ending"},{"id":"bad-arguments"},{"id":"refusing"}]}`))
	case r.Method == http.MethodPost && r.URL.Path == "/v1/chat/completions":
		var req map[string]any
		json.NewDecoder(r.Body).Decode(&req)
		s.Lock()
		s.request = req
		s.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if req["model"] == "missing" {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"error":{"message":"The model does not exist","type":"invalid_request_error"}}`))
			return
		}
		if req["stream"] == true {
			// A streamed reply is chunks of reasoning and text, and the
			// pieces of a tool call, and then a chunk with why the model
			// stopped, and a chunk with the tokens used, or a chunk which
			// fails, or a stream which ends early
			w.Header().Set("Content-Type", "text/event-stream")
			chunks := []string{
				`{"id":"1","model":"deepseek-chat","choices":[{"index":0,"delta":{"role":"assistant","content":null,"reasoning_content":"Hmm, "}}]}`,
				`{"id":"1","model":"deepseek-chat","choices":[{"index":0,"delta":{"reasoning_content":"a greeting."}}]}`,
				`{"id":"1","model":"deepseek-chat","choices":[{"index":0,"delta":{"content":"Hello"}}]}`,
				`{"id":"1","model":"deepseek-chat","choices":[{"index":0,"delta":{"content":" there"}}]}`,
				`{"id":"1","model":"deepseek-chat","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"wave","arguments":""}}]}}]}`,
				`{"id":"1","model":"deepseek-chat","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"hand\":"}}]}}]}`,
				`{"id":"1","model":"deepseek-chat","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"\"left\"}"}}]}}]}`,
			}
			if req["model"] == "refusing" {
				chunks = []string{
					`{"id":"1","choices":[{"index":0,"delta":{"role":"assistant","content":null,"refusal":"I can't "}}]}`,
					`{"id":"1","choices":[{"index":0,"delta":{"refusal":"help with that"}}]}`,
				}
			}
			switch req["model"] {
			case "failing":
				chunks = append(chunks[:3], `{"error":{"message":"the server is overloaded"}}`)
			case "ending":
				chunks = chunks[:3]
			default:
				chunks = append(chunks,
					`{"id":"1","model":"deepseek-chat","choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}]}`,
					`{"id":"1","model":"deepseek-chat","choices":[],"usage":{"prompt_tokens":26,"completion_tokens":5}}`,
					`[DONE]`,
				)
			}
			for _, chunk := range chunks {
				w.Write([]byte("data: " + chunk + "\n\n"))
			}
			return
		}
		if req["model"] == "refusing" {
			w.Write([]byte(`{"id":"1","choices":[{"index":0,"message":{"role":"assistant","content":null,"refusal":"I can't help with that"},"finish_reason":"stop"}],"usage":{"prompt_tokens":26,"completion_tokens":6}}`))
			return
		}
		if req["model"] == "bad-arguments" {
			w.Write([]byte(`{"id":"1","choices":[{"index":0,"message":{"role":"assistant","content":null,"tool_calls":[{"id":"call_1","type":"function","function":{"name":"wave","arguments":"{hand: left"}}]},"finish_reason":"tool_calls"}],"usage":{"prompt_tokens":26,"completion_tokens":4}}`))
			return
		}
		w.Write([]byte(`{
			"id": "1",
			"model": "deepseek-chat",
			"choices": [{"index": 0, "message": {"role": "assistant", "content": "Hello there"}, "finish_reason": "stop"}],
			"usage": {"prompt_tokens": 26, "completion_tokens": 3}
		}`))
	default:
		http.NotFound(w, r)
	}
}

// message is a message to send, with tool calls or results
type message struct {
	role, text string
	calls      []llm.ToolCall
	results    []llm.ToolResult
}

func (m message) Role() string                  { return m.role }
func (m message) Text() string                  { return m.text }
func (m message) Resources() []llm.Resource     { return nil }
func (m message) ToolCalls() []llm.ToolCall     { return m.calls }
func (m message) ToolResults() []llm.ToolResult { return m.results }

func newChatClient(t *testing.T) (*openai.Client, *chatServer) {
	t.Helper()
	stub := new(chatServer)
	server := httptest.NewServer(stub)
	t.Cleanup(server.Close)
	c, err := openai.New(server.URL, apiKey)
	require.NoError(t, err)
	return c, stub
}

func Test_ImplementsGenerator(t *testing.T) {
	assert.Implements(t, (*llm.Generator)(nil), &openai.Client{})
}

func Test_WithoutSession(t *testing.T) {
	c, stub := newChatClient(t)
	ctx := context.Background()
	messages := []llm.Message{
		message{role: "system", text: "Be brief"},
		message{role: "user", text: "Hello"},
	}
	model := func(t *testing.T, name string) llm.Model {
		t.Helper()
		model, err := c.GetModel(ctx, name)
		require.NoError(t, err)
		return model
	}

	t.Run("Reply", func(t *testing.T) {
		reply, usage, err := c.WithoutSession(ctx, model(t, "deepseek-chat"), messages, nil, nil, llm.WithMaxTokens(100))
		require.NoError(t, err)
		assert.Equal(t, "assistant", reply.Role())
		assert.Equal(t, "Hello there", reply.Text())
		assert.Equal(t, llm.Usage{InputTokens: 26, OutputTokens: 3}, usage)

		// Without a stream function, the request isn't streamed, and has
		// the messages, and the options, with the most tokens as max_tokens
		// for servers other than OpenAI
		stub.Lock()
		defer stub.Unlock()
		assert.Equal(t, "deepseek-chat", stub.request["model"])
		assert.Nil(t, stub.request["stream"])
		assert.Equal(t, float64(100), stub.request["max_tokens"])
		assert.Equal(t, []any{
			map[string]any{"role": "system", "content": "Be brief"},
			map[string]any{"role": "user", "content": "Hello"},
		}, stub.request["messages"])
	})

	t.Run("Stream", func(t *testing.T) {
		// The stream function is called with each piece of reasoning and
		// text, and the reply has them all, with the tool call, whose
		// arguments are its pieces, the reason the model stopped, and the
		// tokens used
		var streamed []string
		reply, usage, err := c.WithoutSession(ctx, model(t, "deepseek-chat"), messages, nil, func(what llm.StreamType, text string) {
			streamed = append(streamed, string(what)+":"+text)
		})
		require.NoError(t, err)
		assert.Equal(t, []string{"thinking:Hmm, ", "thinking:a greeting.", "text:Hello", "text: there"}, streamed)
		assert.Equal(t, "assistant", reply.Role())
		assert.Equal(t, "Hello there", reply.Text())
		require.Len(t, reply.ToolCalls(), 1)
		assert.Equal(t, llm.ToolCall{ID: "call_1", Name: "wave", Input: json.RawMessage(`{"hand":"left"}`)}, reply.ToolCalls()[0])
		assert.Equal(t, llm.Usage{InputTokens: 26, OutputTokens: 5}, usage)
		stub.Lock()
		assert.Equal(t, true, stub.request["stream"])
		assert.Equal(t, map[string]any{"include_usage": true}, stub.request["stream_options"])
		stub.Unlock()
	})

	t.Run("StreamError", func(t *testing.T) {
		// An error in the stream, or a stream which ends before it's done,
		// fails the reply
		for _, name := range []string{"failing", "ending"} {
			_, _, err := c.WithoutSession(ctx, model(t, name), messages, nil, func(llm.StreamType, string) {})
			assert.Error(t, err, name)
		}
	})

	t.Run("ToolCalls", func(t *testing.T) {
		// A reply's tool calls are sent with it, with their arguments as a
		// string, and without content, and each result is a tool message
		conversation := append(messages,
			message{role: "thinking", text: "I should wave"},
			message{role: "assistant", calls: []llm.ToolCall{{ID: "call_1", Name: "wave", Input: json.RawMessage(`{"hand":"left"}`)}, {ID: "call_2", Name: "smile"}}},
			message{role: "tool", results: []llm.ToolResult{{ID: "call_1", Name: "wave", Content: json.RawMessage(`"waved"`)}, {ID: "call_2", Name: "smile", Content: json.RawMessage(`"no face"`), IsError: true}}},
		)
		_, _, err := c.WithoutSession(ctx, model(t, "deepseek-chat"), conversation, nil, nil)
		require.NoError(t, err)
		stub.Lock()
		defer stub.Unlock()
		sent := stub.request["messages"].([]any)
		require.Len(t, sent, 5)
		assert.Equal(t, map[string]any{"role": "assistant", "content": nil, "tool_calls": []any{
			map[string]any{"id": "call_1", "type": "function", "function": map[string]any{"name": "wave", "arguments": `{"hand":"left"}`}},
			map[string]any{"id": "call_2", "type": "function", "function": map[string]any{"name": "smile", "arguments": `{}`}},
		}}, sent[2])
		assert.Equal(t, map[string]any{"role": "tool", "content": `"waved"`, "tool_call_id": "call_1"}, sent[3])
		assert.Equal(t, map[string]any{"role": "tool", "content": `"no face"`, "tool_call_id": "call_2"}, sent[4])
	})

	t.Run("BadArguments", func(t *testing.T) {
		// Arguments which aren't JSON are a JSON string, so the tool fails
		reply, _, err := c.WithoutSession(ctx, model(t, "bad-arguments"), messages, nil, nil)
		require.NoError(t, err)
		require.Len(t, reply.ToolCalls(), 1)
		assert.Equal(t, json.RawMessage(`"{hand: left"`), reply.ToolCalls()[0].Input)
	})

	t.Run("Refusal", func(t *testing.T) {
		// A refusal is the reply's text, which is streamed, and the reply is
		// blocked
		reply, _, err := c.WithoutSession(ctx, model(t, "refusing"), messages, nil, nil)
		require.NoError(t, err)
		assert.Equal(t, "I can't help with that", reply.Text())
		assert.Equal(t, schema.ResultBlocked, reply.(*schema.Message).Result)

		var streamed []string
		reply, _, err = c.WithoutSession(ctx, model(t, "refusing"), messages, nil, func(what llm.StreamType, text string) {
			streamed = append(streamed, string(what)+":"+text)
		})
		require.NoError(t, err)
		assert.Equal(t, []string{"text:I can't ", "text:help with that"}, streamed)
		assert.Equal(t, "I can't help with that", reply.Text())
		assert.Equal(t, schema.ResultBlocked, reply.(*schema.Message).Result)
	})

	t.Run("ModelNotFound", func(t *testing.T) {
		_, _, err := c.WithoutSession(ctx, model(t, "missing"), messages, nil, nil)
		assert.ErrorContains(t, err, "does not exist")
	})

	t.Run("OtherProvider", func(t *testing.T) {
		other, err := eliza.New()
		require.NoError(t, err)
		elizaModel, err := other.GetModel(ctx, eliza.ModelName)
		require.NoError(t, err)
		_, _, err = c.WithoutSession(ctx, elizaModel, messages, nil, nil)
		assert.ErrorIs(t, err, llm.ErrBadParameter)
		_, _, err = c.WithoutSession(ctx, nil, messages, nil, nil)
		assert.ErrorIs(t, err, llm.ErrBadParameter)

		// Or a model of another client
		another, _ := newChatClient(t)
		_, _, err = another.WithoutSession(ctx, model(t, "deepseek-chat"), messages, nil, nil)
		assert.ErrorIs(t, err, llm.ErrBadParameter)
	})

	t.Run("NoMessages", func(t *testing.T) {
		_, _, err := c.WithoutSession(ctx, model(t, "deepseek-chat"), nil, nil, nil)
		assert.ErrorIs(t, err, llm.ErrBadParameter)
	})

	t.Run("BadMessage", func(t *testing.T) {
		_, _, err := c.WithoutSession(ctx, model(t, "deepseek-chat"), []llm.Message{message{role: "robot"}}, nil, nil)
		assert.ErrorIs(t, err, llm.ErrBadParameter)
		_, _, err = c.WithoutSession(ctx, model(t, "deepseek-chat"), []llm.Message{message{role: "user", calls: []llm.ToolCall{{Name: "wave"}}}}, nil, nil)
		assert.ErrorIs(t, err, llm.ErrBadParameter)
	})

	t.Run("BadOption", func(t *testing.T) {
		for _, opt := range []llm.Opt{llm.WithTopK(10), llm.WithKeepAlive(-1)} {
			_, _, err := c.WithoutSession(ctx, model(t, "deepseek-chat"), messages, nil, nil, opt)
			assert.ErrorIs(t, err, llm.ErrBadParameter)
			assert.True(t, strings.Contains(err.Error(), "isn't supported"), err.Error())
		}
	})
}
