package schema_test

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/mutablelogic/go-llm/pkg/schema"
	"github.com/stretchr/testify/assert"
)

func TestNewMessage(t *testing.T) {
	assert := assert.New(t)

	// Test basic message creation
	msg, err := schema.NewMessage("user", "Hello, world!")
	assert.NoError(err)
	assert.Equal("user", msg.Role)
	assert.Len(msg.Content, 1)
	assert.Equal("text", msg.Content[0].Type)
	assert.NotNil(msg.Content[0].Text)
	assert.Equal("Hello, world!", *msg.Content[0].Text)
}

func TestMessageText(t *testing.T) {
	assert := assert.New(t)

	// Test Text() method
	msg, err := schema.NewMessage("assistant", "Hello")
	assert.NoError(err)
	assert.Equal("Hello", msg.Text())

	// Test with multiple text blocks
	msg2 := schema.Message{
		Role: "assistant",
		Content: []schema.ContentBlock{
			{Type: "text", Text: ptr("First")},
			{Type: "text", Text: ptr("Second")},
		},
	}
	assert.Equal("First\nSecond", msg2.Text())

	// Test with mixed content types
	msg3 := schema.Message{
		Role: "assistant",
		Content: []schema.ContentBlock{
			{Type: "text", Text: ptr("Hello")},
			{Type: "thinking", Thinking: ptr("reasoning...")},
			{Type: "text", Text: ptr("World")},
		},
	}
	assert.Equal("Hello\nWorld", msg3.Text())
}

func TestMessageMarshalJSON(t *testing.T) {
	assert := assert.New(t)

	// Test marshaling a simple text message
	msg, err := schema.NewMessage("user", "Hello")
	assert.NoError(err)
	data, err := json.Marshal(msg)
	assert.NoError(err)

	expected := `{
		"role": "user",
		"content": [
			{
				"type": "text",
				"text": "Hello"
			}
		],
		"tokens": 0,
		"result": 0
	}`
	assert.JSONEq(expected, string(data))
}

func TestMessageUnmarshalJSON(t *testing.T) {
	assert := assert.New(t)

	// Test unmarshaling a message with content blocks
	jsonData := `{
		"role": "assistant",
		"content": [
			{
				"type": "text",
				"text": "Here's the result"
			},
			{
				"type": "tool_use",
				"tool_use_id": "tool_123",
				"tool_name": "calculator",
				"tool_input": {"x": 1, "y": 2}
			}
		]
	}`

	var msg schema.Message
	err := json.Unmarshal([]byte(jsonData), &msg)
	assert.NoError(err)
	assert.Equal("assistant", msg.Role)
	assert.Len(msg.Content, 2)
	assert.Equal("text", msg.Content[0].Type)
	assert.Equal("tool_use", msg.Content[1].Type)
	assert.NotNil(msg.Content[1].ToolName)
	assert.Equal("calculator", *msg.Content[1].ToolName)
}

func TestContentBlockToolUse(t *testing.T) {
	assert := assert.New(t)

	// Test tool use content block
	toolInput := json.RawMessage(`{"location": "San Francisco"}`)
	block := schema.ContentBlock{
		Type: "tool_use",
		ToolUse: schema.ToolUse{
			ToolUseID: ptr("toolu_123"),
			ToolName:  ptr("get_weather"),
			ToolInput: toolInput,
		},
	}

	data, err := json.Marshal(block)
	assert.NoError(err)

	var decoded schema.ContentBlock
	err = json.Unmarshal(data, &decoded)
	assert.NoError(err)
	assert.Equal("tool_use", decoded.Type)
	assert.Equal("toolu_123", *decoded.ToolUseID)
	assert.Equal("get_weather", *decoded.ToolName)
	assert.JSONEq(`{"location": "San Francisco"}`, string(decoded.ToolInput))
}

func TestContentBlockToolResult(t *testing.T) {
	assert := assert.New(t)

	// Test tool result content block
	resultContent := json.RawMessage(`[{"type":"text","text":"{\"temperature\": 72, \"conditions\": \"sunny\"}"}]`)
	block := schema.ContentBlock{
		Type: "tool_result",
		ToolResult: schema.ToolResult{
			ToolResultID:      ptr("toolu_123"),
			ToolResultContent: resultContent,
			ToolError:         ptr(false),
		},
	}

	data, err := json.Marshal(block)
	assert.NoError(err)

	var decoded schema.ContentBlock
	err = json.Unmarshal(data, &decoded)
	assert.NoError(err)
	assert.Equal("tool_result", decoded.Type)
	assert.Equal("toolu_123", *decoded.ToolResultID)
	assert.False(*decoded.ToolError)
	assert.JSONEq(`[{"type":"text","text":"{\"temperature\": 72, \"conditions\": \"sunny\"}"}]`, string(decoded.ToolResultContent))
}

func TestContentBlockImage(t *testing.T) {
	assert := assert.New(t)

	// Test image content block with base64
	block := schema.ContentBlock{
		Type: "image",
		ImageSource: &schema.ImageSource{
			Type:      "base64",
			MediaType: "image/jpeg",
			Data:      ptr("base64encodeddata..."),
		},
	}

	data, err := json.Marshal(block)
	assert.NoError(err)

	var decoded schema.ContentBlock
	err = json.Unmarshal(data, &decoded)
	assert.NoError(err)
	assert.Equal("image", decoded.Type)
	assert.NotNil(decoded.ImageSource)
	assert.Equal("base64", decoded.ImageSource.Type)
	assert.Equal("image/jpeg", decoded.ImageSource.MediaType)
}

func TestContentBlockDocument(t *testing.T) {
	assert := assert.New(t)

	// Test document content block
	block := schema.ContentBlock{
		Type: "document",
		DocumentSource: &schema.DocumentSource{
			Type:      "base64",
			MediaType: "application/pdf",
			Data:      ptr("pdfdata..."),
		},
		DocumentTitle: ptr("Annual Report"),
	}

	data, err := json.Marshal(block)
	assert.NoError(err)

	var decoded schema.ContentBlock
	err = json.Unmarshal(data, &decoded)
	assert.NoError(err)
	assert.Equal("document", decoded.Type)
	assert.NotNil(decoded.DocumentSource)
	assert.Equal("application/pdf", decoded.DocumentSource.MediaType)
	assert.Equal("Annual Report", *decoded.DocumentTitle)
}

func TestContentBlockThinking(t *testing.T) {
	assert := assert.New(t)

	// Test thinking content block
	block := schema.ContentBlock{
		Type:     "thinking",
		Thinking: ptr("Let me analyze this step by step..."),
	}

	data, err := json.Marshal(block)
	assert.NoError(err)

	var decoded schema.ContentBlock
	err = json.Unmarshal(data, &decoded)
	assert.NoError(err)
	assert.Equal("thinking", decoded.Type)
	assert.Equal("Let me analyze this step by step...", *decoded.Thinking)
}

func TestContentBlockCacheControl(t *testing.T) {
	assert := assert.New(t)

	// Test content block with cache control
	block := schema.ContentBlock{
		Type: "text",
		Text: ptr("This is cached content"),
		CacheControl: &schema.CacheControl{
			Type: "ephemeral",
			TTL:  "5m",
		},
	}

	data, err := json.Marshal(block)
	assert.NoError(err)

	var decoded schema.ContentBlock
	err = json.Unmarshal(data, &decoded)
	assert.NoError(err)
	assert.NotNil(decoded.CacheControl)
	assert.Equal("ephemeral", decoded.CacheControl.Type)
	assert.Equal("5m", decoded.CacheControl.TTL)
}

func TestMessageRoundTrip(t *testing.T) {
	assert := assert.New(t)

	// Create a complex message with multiple content types
	original := schema.Message{
		Role: "assistant",
		Content: []schema.ContentBlock{
			{
				Type:     "thinking",
				Thinking: ptr("I need to search for this..."),
			},
			{
				Type: "text",
				Text: ptr("Let me check that for you."),
			},
			{
				Type: "tool_use",
				ToolUse: schema.ToolUse{
					ToolUseID: ptr("tool_1"),
					ToolName:  ptr("search"),
					ToolInput: json.RawMessage(`{"query": "test"}`),
				},
			},
		},
	}

	// Marshal
	data, err := json.Marshal(original)
	assert.NoError(err)

	// Unmarshal
	var decoded schema.Message
	err = json.Unmarshal(data, &decoded)
	assert.NoError(err)

	// Verify
	assert.Equal(original.Role, decoded.Role)
	assert.Len(decoded.Content, 3)
	assert.Equal("thinking", decoded.Content[0].Type)
	assert.Equal("text", decoded.Content[1].Type)
	assert.Equal("tool_use", decoded.Content[2].Type)
}

func TestAppendToolResult_Success(t *testing.T) {
	assert := assert.New(t)

	// Create a tool message
	msg := schema.NewToolMessage()
	assert.Equal("user", msg.Role)
	assert.Empty(msg.Content)

	// Create a tool use to respond to
	toolUse := schema.ToolUse{
		ToolUseID: ptr("toolu_123"),
		ToolName:  ptr("get_weather"),
		ToolInput: json.RawMessage(`{"location": "SF"}`),
	}

	// Append a successful result
	result := map[string]interface{}{
		"temperature": 72,
		"conditions":  "sunny",
	}
	err := msg.AppendToolResult(toolUse, result)
	assert.NoError(err)
	assert.Len(msg.Content, 1)

	// Verify the content block
	block := msg.Content[0]
	assert.Equal("tool_result", block.Type)
	assert.Equal("toolu_123", *block.ToolResultID)
	assert.NotNil(block.ToolError)
	assert.False(*block.ToolError)
	var content []map[string]string
	assert.NoError(json.Unmarshal(block.ToolResultContent, &content))
	assert.Len(content, 1)
	assert.Equal("text", content[0]["type"])
	assert.JSONEq(`{"temperature":72,"conditions":"sunny"}`, content[0]["text"])
}

func TestAppendToolResult_Error(t *testing.T) {
	assert := assert.New(t)

	msg := schema.NewToolMessage()
	toolUse := schema.ToolUse{
		ToolUseID: ptr("toolu_456"),
		ToolName:  ptr("calculator"),
	}

	// Append an error result
	testErr := fmt.Errorf("calculation failed")
	err := msg.AppendToolResult(toolUse, testErr)
	assert.NoError(err)
	assert.Len(msg.Content, 1)

	// Verify error flag is set
	block := msg.Content[0]
	assert.Equal("tool_result", block.Type)
	assert.NotNil(block.ToolError)
	assert.True(*block.ToolError)
}

func TestAppendToolResult_Nil(t *testing.T) {
	assert := assert.New(t)

	msg := schema.NewToolMessage()
	toolUse := schema.ToolUse{
		ToolUseID: ptr("toolu_789"),
	}

	// Append nil result
	err := msg.AppendToolResult(toolUse, nil)
	assert.NoError(err)
	assert.Len(msg.Content, 1)

	block := msg.Content[0]
	var content []map[string]string
	assert.NoError(json.Unmarshal(block.ToolResultContent, &content))
	assert.Len(content, 1)
	assert.Equal("text", content[0]["type"])
	assert.Equal("null", content[0]["text"])
	assert.NotNil(block.ToolError)
	assert.False(*block.ToolError)
}

func TestAppendToolResult_RawJSON(t *testing.T) {
	assert := assert.New(t)

	msg := schema.NewToolMessage()
	toolUse := schema.ToolUse{
		ToolUseID: ptr("toolu_raw"),
	}

	// Append raw JSON
	raw := json.RawMessage(`{"raw": "data"}`)
	err := msg.AppendToolResult(toolUse, raw)
	assert.NoError(err)
	assert.Len(msg.Content, 1)

	block := msg.Content[0]
	var content []map[string]string
	assert.NoError(json.Unmarshal(block.ToolResultContent, &content))
	assert.Len(content, 1)
	assert.Equal("text", content[0]["type"])
	assert.JSONEq(`{"raw":"data"}`, content[0]["text"])
}

func TestAppendToolResult_WrongRole(t *testing.T) {
	assert := assert.New(t)

	// Create an assistant message (wrong role for tool results)
	msg, err := schema.NewMessage("assistant", "Hello")
	assert.NoError(err)

	toolUse := schema.ToolUse{
		ToolUseID: ptr("toolu_fail"),
	}

	// Attempt to append tool result should fail
	err = msg.AppendToolResult(toolUse, "result")
	assert.Error(err)
	assert.Contains(err.Error(), "cannot append tool result to non-user message")
}

func TestAppendToolResult_Multiple(t *testing.T) {
	assert := assert.New(t)

	msg := schema.NewToolMessage()

	// Append first tool result
	toolUse1 := schema.ToolUse{ToolUseID: ptr("tool_1")}
	err := msg.AppendToolResult(toolUse1, "result1")
	assert.NoError(err)

	// Append second tool result
	toolUse2 := schema.ToolUse{ToolUseID: ptr("tool_2")}
	err = msg.AppendToolResult(toolUse2, "result2")
	assert.NoError(err)

	// Verify both results are present
	assert.Len(msg.Content, 2)
	assert.Equal("tool_1", *msg.Content[0].ToolResultID)
	assert.Equal("tool_2", *msg.Content[1].ToolResultID)
}

// Helper function
func ptr[T any](v T) *T {
	return &v
}
