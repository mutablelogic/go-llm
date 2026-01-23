package schema_test

import (
	"encoding/json"
	"testing"

	"github.com/mutablelogic/go-llm/pkg/schema"
	"github.com/stretchr/testify/assert"
)

func TestNewMessage(t *testing.T) {
	assert := assert.New(t)

	// Test basic message creation
	msg := schema.NewMessage("user", "Hello, world!")
	assert.Equal("user", msg.Role)
	assert.Len(msg.Content, 1)
	assert.Equal("text", msg.Content[0].Type)
	assert.NotNil(msg.Content[0].Text)
	assert.Equal("Hello, world!", *msg.Content[0].Text)
}

func TestMessageText(t *testing.T) {
	assert := assert.New(t)

	// Test Text() method
	msg := schema.NewMessage("assistant", "Hello")
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
	msg := schema.NewMessage("user", "Hello")
	data, err := json.Marshal(msg)
	assert.NoError(err)

	expected := `{
		"role": "user",
		"content": [
			{
				"type": "text",
				"text": "Hello"
			}
		]
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
		Type:      "tool_use",
		ToolUseID: ptr("toolu_123"),
		ToolName:  ptr("get_weather"),
		ToolInput: toolInput,
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
	resultContent := json.RawMessage(`{"temperature": 72, "conditions": "sunny"}`)
	block := schema.ContentBlock{
		Type:              "tool_result",
		ToolResultID:      ptr("toolu_123"),
		ToolResultContent: resultContent,
		IsError:           ptr(false),
	}

	data, err := json.Marshal(block)
	assert.NoError(err)

	var decoded schema.ContentBlock
	err = json.Unmarshal(data, &decoded)
	assert.NoError(err)
	assert.Equal("tool_result", decoded.Type)
	assert.Equal("toolu_123", *decoded.ToolResultID)
	assert.False(*decoded.IsError)
	assert.JSONEq(`{"temperature": 72, "conditions": "sunny"}`, string(decoded.ToolResultContent))
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
				Type:      "tool_use",
				ToolUseID: ptr("tool_1"),
				ToolName:  ptr("search"),
				ToolInput: json.RawMessage(`{"query": "test"}`),
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

// Helper function
func ptr[T any](v T) *T {
	return &v
}
