package schema_test

import (
	"testing"

	// Packages
	schema "github.com/mutablelogic/go-llm/provider/schema"
	assert "github.com/stretchr/testify/assert"
)

func Test_CodeFence(t *testing.T) {
	for text, fence := range map[string]string{
		"":                     "```",
		"no backticks":         "```",
		"`inline` and ``two``": "```",
		"```go\n```":           "````",
		"````\n```\n````":      "`````",
	} {
		assert.Equal(t, fence, schema.CodeFence(text), text)
	}
}

func Test_IsText(t *testing.T) {
	assert.True(t, schema.IsText([]byte("hello"), "text/plain"))
	assert.True(t, schema.IsText([]byte(`{"a":1}`), "application/json"))
	assert.True(t, schema.IsText(nil, ""))
	assert.False(t, schema.IsText([]byte("hello"), "image/png"))
	assert.False(t, schema.IsText([]byte("%PDF-1.7"), "application/pdf"))
	assert.False(t, schema.IsText([]byte{0xff, 0xfe, 0}, "application/octet-stream"))
}

func Test_TextBlock(t *testing.T) {
	block := schema.TextBlock("notes.md", []byte("```go\n```\n"))
	if assert.NotNil(t, block.Text) {
		assert.Equal(t, "notes.md:\n\n````\n```go\n```\n````", *block.Text)
	}
}
