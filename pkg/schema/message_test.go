package schema_test

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	// Packages
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
	assert "github.com/stretchr/testify/assert"
)

// testdataPath returns the absolute path to the etc/testdata directory
func testdataPath(name string) string {
	_, filename, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(filename), "..", "..", "etc", "testdata", name)
}

func Test_NewMessage_001(t *testing.T) {
	// Simple text message
	assert := assert.New(t)
	msg, err := schema.NewMessage("user", "Hello, world!")
	assert.NoError(err)
	assert.NotNil(msg)
	assert.Equal("user", msg.Role)
	assert.Len(msg.Content, 1)
	assert.NotNil(msg.Content[0].Text)
	assert.Equal("Hello, world!", *msg.Content[0].Text)
	assert.Equal("Hello, world!", msg.Text())
}

func Test_NewMessage_002(t *testing.T) {
	// Assistant text message
	assert := assert.New(t)
	msg, err := schema.NewMessage("assistant", "I can help with that.")
	assert.NoError(err)
	assert.Equal("assistant", msg.Role)
	assert.Equal("I can help with that.", msg.Text())
}

func Test_NewMessage_003(t *testing.T) {
	// System text message
	assert := assert.New(t)
	msg, err := schema.NewMessage("system", "You are a helpful assistant.")
	assert.NoError(err)
	assert.Equal("system", msg.Role)
	assert.Equal("You are a helpful assistant.", msg.Text())
}

func Test_NewMessage_004(t *testing.T) {
	// Empty text message
	assert := assert.New(t)
	msg, err := schema.NewMessage("user", "")
	assert.NoError(err)
	assert.NotNil(msg)
	assert.Equal("", msg.Text())
}

func Test_NewMessage_005(t *testing.T) {
	// Text with attachment (image file)
	assert := assert.New(t)

	f, err := os.Open(testdataPath("guggenheim.jpg"))
	if !assert.NoError(err) {
		t.FailNow()
	}
	defer f.Close()

	msg, err := schema.NewMessage("user", "What is in this image?", schema.WithAttachment(f))
	assert.NoError(err)
	assert.NotNil(msg)
	assert.Equal("user", msg.Role)
	assert.Len(msg.Content, 2)

	// First block is text
	assert.NotNil(msg.Content[0].Text)
	assert.Equal("What is in this image?", *msg.Content[0].Text)

	// Second block is attachment
	assert.NotNil(msg.Content[1].Attachment)
	assert.True(strings.HasPrefix(msg.Content[1].Attachment.Type, "image/jpeg"))
	assert.Greater(len(msg.Content[1].Attachment.Data), 0)
	assert.Nil(msg.Content[1].Attachment.URL)
}

func Test_NewMessage_006(t *testing.T) {
	// Text method concatenates multiple text blocks
	assert := assert.New(t)
	msg := &schema.Message{
		Role: "assistant",
		Content: []schema.ContentBlock{
			{Text: types.Ptr("Hello")},
			{Text: types.Ptr("World")},
		},
	}
	assert.Equal("Hello\nWorld", msg.Text())
}

func Test_NewMessage_007(t *testing.T) {
	// ToolCalls returns tool call blocks
	assert := assert.New(t)
	msg := &schema.Message{
		Role: "assistant",
		Content: []schema.ContentBlock{
			{Text: types.Ptr("Let me check that")},
			{ToolCall: &schema.ToolCall{ID: "call_1", Name: "get_weather"}},
			{ToolCall: &schema.ToolCall{ID: "call_2", Name: "get_time"}},
		},
	}
	calls := msg.ToolCalls()
	assert.Len(calls, 2)
	assert.Equal("get_weather", calls[0].Name)
	assert.Equal("get_time", calls[1].Name)
}

func Test_NewMessage_008(t *testing.T) {
	// String method doesn't panic
	assert := assert.New(t)
	msg, err := schema.NewMessage("user", "test")
	assert.NoError(err)
	assert.NotEmpty(msg.String())
}

func Test_NewMessage_009(t *testing.T) {
	// Text with URL attachment
	assert := assert.New(t)

	msg, err := schema.NewMessage("user", "Describe this image", schema.WithAttachmentURL("gs://my-bucket/image.png", "image/png"))
	assert.NoError(err)
	assert.NotNil(msg)
	assert.Len(msg.Content, 2)

	// First block is text
	assert.NotNil(msg.Content[0].Text)
	assert.Equal("Describe this image", *msg.Content[0].Text)

	// Second block is URL attachment
	att := msg.Content[1].Attachment
	assert.NotNil(att)
	assert.Equal("image/png", att.Type)
	assert.Nil(att.Data)
	assert.NotNil(att.URL)
	assert.Equal("gs://my-bucket/image.png", att.URL.String())
}

func Test_NewMessage_010(t *testing.T) {
	// Multiple attachments on one message
	assert := assert.New(t)

	f, err := os.Open(testdataPath("guggenheim.jpg"))
	if !assert.NoError(err) {
		t.FailNow()
	}
	defer f.Close()

	msg, err := schema.NewMessage("user", "Compare these images",
		schema.WithAttachment(f),
		schema.WithAttachmentURL("https://example.com/photo.png", "image/png"),
	)
	assert.NoError(err)
	assert.NotNil(msg)
	assert.Len(msg.Content, 3)

	// First block is text
	assert.NotNil(msg.Content[0].Text)
	assert.Equal("Compare these images", *msg.Content[0].Text)

	// Second block is inline data attachment
	assert.NotNil(msg.Content[1].Attachment)
	assert.True(strings.HasPrefix(msg.Content[1].Attachment.Type, "image/jpeg"))
	assert.Greater(len(msg.Content[1].Attachment.Data), 0)

	// Third block is URL attachment
	assert.NotNil(msg.Content[2].Attachment)
	assert.Equal("image/png", msg.Content[2].Attachment.Type)
	assert.Equal("https://example.com/photo.png", msg.Content[2].Attachment.URL.String())
}

func Test_NewToolResult_001(t *testing.T) {
	// Simple tool result
	assert := assert.New(t)
	content := map[string]any{"temperature": 20, "unit": "celsius"}
	block := schema.NewToolResult("call_123", "get_weather", content)

	tr := block.ToolResult
	assert.NotNil(tr)
	assert.Equal("call_123", tr.ID)
	assert.Equal("get_weather", tr.Name)
	assert.JSONEq(`{"temperature":20,"unit":"celsius"}`, string(tr.Content))
	assert.False(tr.IsError)
}

func Test_NewToolError_001(t *testing.T) {
	// Tool error
	assert := assert.New(t)
	block := schema.NewToolError("call_456", "get_weather", errors.New("city not found"))

	tr := block.ToolResult
	assert.NotNil(tr)
	assert.Equal("call_456", tr.ID)
	assert.Equal("get_weather", tr.Name)
	assert.True(tr.IsError)
	assert.Contains(string(tr.Content), "city not found")
}
