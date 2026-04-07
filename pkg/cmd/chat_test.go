package cmd

import (
	"testing"

	// Packages
	uuid "github.com/google/uuid"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	assert "github.com/stretchr/testify/assert"
)

func TestChatCommandRequest(t *testing.T) {
	assert := assert.New(t)
	session := uuid.MustParse("11111111-1111-1111-1111-111111111111")

	req := (ChatCommand{
		Session:       session,
		Text:          "hello there",
		Tools:         []string{"builtin.alpha", "builtin.bravo"},
		MaxIterations: 3,
		SystemPrompt:  "reply concisely",
	}).request()

	assert.Equal(schema.ChatRequest{
		Session:       session,
		Text:          "hello there",
		Tools:         []string{"builtin.alpha", "builtin.bravo"},
		MaxIterations: 3,
		SystemPrompt:  "reply concisely",
	}, req)
}

func TestChatResponseText(t *testing.T) {
	assert := assert.New(t)
	response := &schema.ChatResponse{
		CompletionResponse: schema.CompletionResponse{
			Content: []schema.ContentBlock{
				{Text: stringPtr("Hello")},
				{Text: stringPtr(" world")},
			},
		},
	}

	assert.Equal("Hello world", chatResponseText(response))
}

func stringPtr(value string) *string {
	return &value
}
