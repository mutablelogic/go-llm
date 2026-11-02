package openai

import (
	"context"
	"testing"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

// embedMessage is a message to embed
type embedMessage string

func (m embedMessage) Role() string                  { return "user" }
func (m embedMessage) Text() string                  { return string(m) }
func (m embedMessage) Resources() []llm.Resource     { return nil }
func (m embedMessage) ToolCalls() []llm.ToolCall     { return nil }
func (m embedMessage) ToolResults() []llm.ToolResult { return nil }

func TestEmbedDeepSeek(t *testing.T) {
	// DeepSeek has no embeddings, so there's no request
	c, err := New(DeepSeekEndpoint, "sk-test")
	require.NoError(t, err)
	model := &Model{provider: c, data: apiModel{ID: "deepseek-chat"}}
	_, _, err = c.Embed(context.Background(), model, []llm.Message{embedMessage("Hello")})
	assert.ErrorIs(t, err, llm.ErrNotImplemented)
}
