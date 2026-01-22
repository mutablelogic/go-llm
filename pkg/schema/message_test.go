package schema_test

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/mutablelogic/go-llm/pkg/schema"
	"github.com/stretchr/testify/assert"
)

func TestStringMessage(t *testing.T) {
	assert := assert.New(t)

	// Test basic user message
	msg := schema.StringMessage("user", "Hello, world!")
	data, err := json.Marshal(msg)
	assert.NoError(err)

	// Should produce: {"role":"user","content":"Hello, world!"}
	expected := `{"role":"user","content":"Hello, world!"}`
	assert.JSONEq(expected, string(data))

	// Unmarshal and verify
	var decoded schema.Message
	err = json.Unmarshal(data, &decoded)
	assert.NoError(err)
	assert.Equal("user", decoded.Role)
}

func TestAssistantMessage(t *testing.T) {
	assert := assert.New(t)

	// Test assistant message
	msg := schema.StringMessage("assistant", "I can help you with that.")
	data, err := json.Marshal(msg)
	assert.NoError(err)

	expected := `{"role":"assistant","content":"I can help you with that."}`
	assert.JSONEq(expected, string(data))
}

func TestSystemMessage(t *testing.T) {
	assert := assert.New(t)

	// Test system message
	msg := schema.StringMessage("system", "You are a helpful assistant.")
	data, err := json.Marshal(msg)
	assert.NoError(err)

	expected := `{"role":"system","content":"You are a helpful assistant."}`
	assert.JSONEq(expected, string(data))
}

func TestToolResultMessage(t *testing.T) {
	assert := assert.New(t)

	// Test successful tool result
	msg := schema.ToolResultMessage("tool_123", `{"result": "success"}`, false)
	data, err := json.Marshal(msg)
	assert.NoError(err)

	// Anthropic format for tool results
	var result map[string]any
	err = json.Unmarshal(data, &result)
	assert.NoError(err)
	assert.Equal("user", result["role"])

	content := result["content"].([]any)
	assert.Len(content, 1)

	toolResult := content[0].(map[string]any)
	assert.Equal("tool_result", toolResult["type"])
	assert.Equal("tool_123", toolResult["tool_use_id"])
}

func TestToolResultMessageWithError(t *testing.T) {
	assert := assert.New(t)

	// Test tool result with error
	msg := schema.ToolResultMessage("tool_456", "Something went wrong", true)
	data, err := json.Marshal(msg)
	assert.NoError(err)

	var result map[string]any
	err = json.Unmarshal(data, &result)
	assert.NoError(err)

	content := result["content"].([]any)
	toolResult := content[0].(map[string]any)
	assert.Equal(true, toolResult["is_error"])
}

func TestImageMessage(t *testing.T) {
	assert := assert.New(t)

	// Create a minimal PNG (1x1 transparent pixel)
	pngData := []byte{
		0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, // PNG signature
		0x00, 0x00, 0x00, 0x0D, 0x49, 0x48, 0x44, 0x52, // IHDR chunk
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x06, 0x00, 0x00, 0x00, 0x1F, 0x15, 0xC4,
		0x89, 0x00, 0x00, 0x00, 0x0A, 0x49, 0x44, 0x41,
		0x54, 0x78, 0x9C, 0x63, 0x00, 0x01, 0x00, 0x00,
		0x05, 0x00, 0x01, 0x0D, 0x0A, 0x2D, 0xB4, 0x00,
		0x00, 0x00, 0x00, 0x49, 0x45, 0x4E, 0x44, 0xAE,
		0x42, 0x60, 0x82,
	}

	msg, err := schema.ImageMessage(bytes.NewReader(pngData), "")
	assert.NoError(err)

	data, err := json.Marshal(msg)
	assert.NoError(err)

	var result map[string]any
	err = json.Unmarshal(data, &result)
	assert.NoError(err)
	assert.Equal("user", result["role"])

	content := result["content"].([]any)
	assert.Len(content, 1)

	imageContent := content[0].(map[string]any)
	assert.Equal("image", imageContent["type"])

	source := imageContent["source"].(map[string]any)
	assert.Equal("base64", source["type"])
	assert.Equal("image/png", source["media_type"])
	assert.NotEmpty(source["data"])
}

func TestImageMessageWithExplicitMediaType(t *testing.T) {
	assert := assert.New(t)

	// Test with explicit media type
	imageData := []byte{0xFF, 0xD8, 0xFF} // JPEG magic bytes
	msg, err := schema.ImageMessage(bytes.NewReader(imageData), "image/jpeg")
	assert.NoError(err)

	data, err := json.Marshal(msg)
	assert.NoError(err)

	var result map[string]any
	err = json.Unmarshal(data, &result)
	assert.NoError(err)

	content := result["content"].([]any)
	imageContent := content[0].(map[string]any)
	source := imageContent["source"].(map[string]any)
	assert.Equal("image/jpeg", source["media_type"])
}

func TestImageMessageInvalidType(t *testing.T) {
	assert := assert.New(t)

	// Test with non-image data
	textData := []byte("This is not an image")
	_, err := schema.ImageMessage(bytes.NewReader(textData), "")
	assert.Error(err)
	assert.Contains(err.Error(), "invalid image type")
}

func TestMessageUnmarshalStringContent(t *testing.T) {
	assert := assert.New(t)

	// Unmarshal message with string content (common format)
	jsonData := `{"role":"user","content":"Hello"}`
	var msg schema.Message
	err := json.Unmarshal([]byte(jsonData), &msg)
	assert.NoError(err)
	assert.Equal("user", msg.Role)
}

func TestMessageUnmarshalArrayContent(t *testing.T) {
	assert := assert.New(t)

	// Unmarshal message with array of text content
	jsonData := `{"role":"user","content":["Hello","World"]}`
	var msg schema.Message
	err := json.Unmarshal([]byte(jsonData), &msg)
	assert.NoError(err)
	assert.Equal("user", msg.Role)
}

func TestMessageUnmarshalTypedContent(t *testing.T) {
	assert := assert.New(t)

	// Unmarshal message with typed content array (Anthropic format)
	jsonData := `{
		"role": "assistant",
		"content": [
			{"type": "text", "text": "Here is the result"},
			{"type": "tool_use", "id": "tool_1", "name": "calculator", "input": {"x": 1, "y": 2}}
		]
	}`
	var msg schema.Message
	err := json.Unmarshal([]byte(jsonData), &msg)
	assert.NoError(err)
	assert.Equal("assistant", msg.Role)
}

func TestMessageRoundTrip(t *testing.T) {
	assert := assert.New(t)

	// Test that messages can be marshaled and unmarshaled
	original := schema.StringMessage("user", "Test message")
	data, err := json.Marshal(original)
	assert.NoError(err)

	var decoded schema.Message
	err = json.Unmarshal(data, &decoded)
	assert.NoError(err)
	assert.Equal(original.Role, decoded.Role)
}
