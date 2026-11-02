package ollama

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/provider/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

// testMessage is a message to convert
type testMessage struct {
	role        string
	text        string
	resources   []llm.Resource
	toolCalls   []llm.ToolCall
	toolResults []llm.ToolResult
}

var _ llm.Message = testMessage{}

func (m testMessage) Role() string                  { return m.role }
func (m testMessage) Text() string                  { return m.text }
func (m testMessage) Resources() []llm.Resource     { return m.resources }
func (m testMessage) ToolCalls() []llm.ToolCall     { return m.toolCalls }
func (m testMessage) ToolResults() []llm.ToolResult { return m.toolResults }

// testResource is a resource to convert, which returns the data or error when read
type testResource struct {
	contentType string
	data        []byte
	err         error
}

var _ llm.Resource = testResource{}

func (r testResource) URI() string         { return "" }
func (r testResource) Name() string        { return "" }
func (r testResource) Description() string { return "" }
func (r testResource) Type() string        { return r.contentType }
func (r testResource) Read(context.Context) (llm.ReadFunc, error) {
	if r.err != nil {
		return nil, r.err
	}
	return llm.ReadBytes(r.data), nil
}

func TestChatMessages(t *testing.T) {
	ctx := context.Background()

	t.Run("Roles", func(t *testing.T) {
		chat, err := chatMessages(ctx, []llm.Message{
			testMessage{role: "system", text: "Be brief"},
			testMessage{role: "user", text: "Hello"},
			testMessage{role: "thinking", text: "They said hello"},
			testMessage{role: "assistant", text: "Hi"},
			testMessage{role: "tool", text: `{"temperature_c":18}`},
		})
		require.NoError(t, err)
		assert.Equal(t, []chatMessage{
			{Role: "system", Content: "Be brief"},
			{Role: "user", Content: "Hello"},
			{Role: "assistant", Thinking: "They said hello"},
			{Role: "assistant", Content: "Hi"},
			{Role: "tool", Content: `{"temperature_c":18}`},
		}, chat)
	})

	t.Run("ToolCalls", func(t *testing.T) {
		// The model's tool calls, and the tools' results, which are a tool
		// message each
		chat, err := chatMessages(ctx, []llm.Message{
			testMessage{role: "assistant", toolCalls: []llm.ToolCall{
				{ID: "call_1", Name: "weather_current", Input: json.RawMessage(`{"city":"London"}`)},
				{ID: "call_2", Name: "weather_alerts"},
			}},
			testMessage{role: "tool", toolResults: []llm.ToolResult{
				{ID: "call_1", Name: "weather_current", Content: json.RawMessage(`{"temperature_c":18}`)},
				{ID: "call_2", Name: "weather_alerts", Content: json.RawMessage(`"not found"`), IsError: true},
			}},
		})
		require.NoError(t, err)
		assert.Equal(t, []chatMessage{
			{Role: "assistant", ToolCalls: []chatToolCall{
				{ID: "call_1", Function: chatToolCallFunction{Name: "weather_current", Arguments: json.RawMessage(`{"city":"London"}`)}},
				{ID: "call_2", Function: chatToolCallFunction{Name: "weather_alerts", Arguments: json.RawMessage(`{}`)}},
			}},
			{Role: "tool", Content: `{"temperature_c":18}`, ToolName: "weather_current", ToolCallID: "call_1"},
			{Role: "tool", Content: `"not found"`, ToolName: "weather_alerts", ToolCallID: "call_2"},
		}, chat)
	})

	t.Run("ToolCallsInUserMessage", func(t *testing.T) {
		_, err := chatMessages(ctx, []llm.Message{
			testMessage{role: "user", toolCalls: []llm.ToolCall{{ID: "call_1", Name: "weather_current"}}},
		})
		assert.ErrorIs(t, err, llm.ErrBadParameter)
	})

	t.Run("Images", func(t *testing.T) {
		chat, err := chatMessages(ctx, []llm.Message{
			testMessage{role: "user", text: "What are these?", resources: []llm.Resource{
				testResource{contentType: "image/png", data: []byte("png")},
				testResource{contentType: "image/jpeg; charset=binary", data: []byte("jpeg")},
			}},
		})
		require.NoError(t, err)
		require.Len(t, chat, 1)
		assert.Equal(t, [][]byte{[]byte("png"), []byte("jpeg")}, chat[0].Images)
	})

	t.Run("UnsupportedRole", func(t *testing.T) {
		_, err := chatMessages(ctx, []llm.Message{testMessage{role: "robot", text: "Beep"}})
		assert.ErrorIs(t, err, llm.ErrBadParameter)
	})

	t.Run("UnsupportedResource", func(t *testing.T) {
		_, err := chatMessages(ctx, []llm.Message{
			testMessage{role: "user", resources: []llm.Resource{testResource{contentType: "application/pdf"}}},
		})
		assert.ErrorIs(t, err, llm.ErrBadParameter)
	})

	t.Run("ReadError", func(t *testing.T) {
		readErr := errors.New("can't read")
		_, err := chatMessages(ctx, []llm.Message{
			testMessage{role: "user", resources: []llm.Resource{testResource{contentType: "image/png", err: readErr}}},
		})
		assert.ErrorIs(t, err, readErr)
	})

	t.Run("Nil", func(t *testing.T) {
		_, err := chatMessages(ctx, []llm.Message{nil})
		assert.ErrorIs(t, err, llm.ErrBadParameter)
	})
}

func TestNewChatResponse(t *testing.T) {
	ctx := context.Background()
	png := []byte("\x89PNG\r\n\x1a\n")

	t.Run("Text", func(t *testing.T) {
		message := newChatResponse(chatResponse{
			Message:     chatMessage{Role: "assistant", Content: "Hello", Thinking: "They said hello"},
			Done:        true,
			DoneReason:  "stop",
			chatMetrics: chatMetrics{PromptEvalCount: 26, EvalCount: 3},
		})
		assert.Equal(t, &schema.Message{
			Sender: "assistant",
			Content: []schema.ContentBlock{
				{Thinking: types.Ptr("They said hello")},
				{Text: types.Ptr("Hello")},
			},
			Tokens: 3,
			Result: schema.ResultStop,
		}, message)
	})

	t.Run("Images", func(t *testing.T) {
		message := newChatResponse(chatResponse{
			Message: chatMessage{Role: "assistant", Images: [][]byte{png}},
		})
		require.Len(t, message.Content, 1)
		assert.Equal(t, &schema.Resource{ResourceMeta: schema.ResourceMeta{ContentType: "image/png", Data: png}}, message.Content[0].Resource)
	})

	t.Run("ToolCall", func(t *testing.T) {
		message := newChatResponse(chatResponse{
			Message: chatMessage{Role: "assistant", ToolCalls: []chatToolCall{{
				ID:       "call_1",
				Function: chatToolCallFunction{Name: "get_weather", Arguments: json.RawMessage(`{"city":"London"}`)},
			}}},
			DoneReason: "stop",
		})
		require.Len(t, message.Content, 1)
		assert.Equal(t, &schema.ToolCall{ID: "call_1", Name: "get_weather", Input: json.RawMessage(`{"city":"London"}`)}, message.Content[0].ToolCall)
		assert.Equal(t, schema.ResultToolCall, message.Result)
	})

	t.Run("Result", func(t *testing.T) {
		for reason, want := range map[string]schema.ResultType{
			"stop":   schema.ResultStop,
			"length": schema.ResultMaxTokens,
			"load":   schema.ResultOther,
			"":       schema.ResultOther,
		} {
			assert.Equal(t, want, newChatResponse(chatResponse{DoneReason: reason}).Result, reason)
		}
	})

	t.Run("RoundTrip", func(t *testing.T) {
		chat, err := chatMessages(ctx, []llm.Message{newChatResponse(chatResponse{
			Message: chatMessage{Role: "assistant", Content: "Hello", Images: [][]byte{png}},
		})})
		require.NoError(t, err)
		assert.Equal(t, []chatMessage{{Role: "assistant", Content: "Hello", Images: [][]byte{png}}}, chat)
	})
}
