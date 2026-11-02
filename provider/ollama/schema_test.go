package ollama

import (
	"encoding/json"
	"testing"
	"time"

	// Packages
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

func TestChatRequest(t *testing.T) {
	stream := true
	data, err := json.Marshal(chatRequest{
		Model:  "llama3.2:latest",
		Stream: &stream,
		Messages: []chatMessage{
			{Role: "system", Content: "Be brief"},
			{Role: "user", Content: "What is this?", Images: [][]byte{[]byte("hello")}},
		},
		Options: map[string]any{"num_ctx": 8192},
	})
	require.NoError(t, err)
	assert.JSONEq(t, `{
		"model": "llama3.2:latest",
		"stream": true,
		"messages": [
			{"role": "system", "content": "Be brief"},
			{"role": "user", "content": "What is this?", "images": ["aGVsbG8="]}
		],
		"options": {"num_ctx": 8192}
	}`, string(data))
}

func TestChatResponse(t *testing.T) {
	t.Run("Streamed", func(t *testing.T) {
		var response chatResponse
		require.NoError(t, json.Unmarshal([]byte(`{
			"model": "llama3.2:latest",
			"created_at": "2026-10-11T08:52:19.385406455Z",
			"message": {"role": "assistant", "content": "The"},
			"done": false
		}`), &response))
		assert.Equal(t, "assistant", response.Message.Role)
		assert.Equal(t, "The", response.Message.Content)
		assert.False(t, response.Done)
	})

	t.Run("Done", func(t *testing.T) {
		var response chatResponse
		require.NoError(t, json.Unmarshal([]byte(`{
			"model": "llama3.2:latest",
			"created_at": "2026-10-11T08:52:19.385406455Z",
			"message": {"role": "assistant", "content": ""},
			"done": true,
			"done_reason": "stop",
			"total_duration": 4883583458,
			"load_duration": 1334875,
			"prompt_eval_count": 26,
			"prompt_eval_duration": 342546000,
			"eval_count": 282,
			"eval_duration": 4535599000
		}`), &response))
		assert.True(t, response.Done)
		assert.Equal(t, "stop", response.DoneReason)
		assert.Equal(t, 26, response.PromptEvalCount)
		assert.Equal(t, 282, response.EvalCount)
		assert.Equal(t, 4883583458*time.Nanosecond, response.TotalDuration)
	})

	t.Run("ToolCall", func(t *testing.T) {
		var response chatResponse
		require.NoError(t, json.Unmarshal([]byte(`{
			"model": "llama3.2:latest",
			"message": {
				"role": "assistant",
				"content": "",
				"tool_calls": [{"function": {"index": 0, "name": "get_weather", "arguments": {"city": "London"}}}]
			},
			"done": true,
			"done_reason": "stop"
		}`), &response))
		require.Len(t, response.Message.ToolCalls, 1)
		assert.Equal(t, "get_weather", response.Message.ToolCalls[0].Function.Name)
		assert.JSONEq(t, `{"city":"London"}`, string(response.Message.ToolCalls[0].Function.Arguments))
	})
}
