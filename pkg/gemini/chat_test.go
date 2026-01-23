package gemini_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	// Packages
	assert "github.com/stretchr/testify/assert"

	"github.com/mutablelogic/go-llm/pkg/gemini"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
)

func Test_chat_001(t *testing.T) {
	assert := assert.New(t)

	// Create a simple session with a user message
	session := &schema.Session{}
	session.Append(schema.StringMessage("user", "Say hello in exactly 3 words"))

	// Send the chat request
	response, err := client.Chat(context.TODO(), "gemini-2.0-flash", session)
	if err != nil {
		if strings.Contains(err.Error(), "429") || strings.Contains(err.Error(), "quota") {
			t.Skip("Skipping due to rate limiting")
		}
		assert.NoError(err)
		return
	}
	assert.NotNil(response)

	// Log the response
	data, err := json.MarshalIndent(response, "", "  ")
	assert.NoError(err)
	t.Log("Response:", string(data))

	// Check session was updated
	assert.Len(*session, 2)
	t.Log("Session tokens:", session.Tokens())
}

func Test_chat_002(t *testing.T) {
	assert := assert.New(t)

	// Test with system prompt
	session := &schema.Session{}
	session.Append(schema.StringMessage("user", "What is 2+2?"))

	// Send with options
	response, err := client.Chat(context.TODO(), "gemini-2.0-flash", session,
		gemini.WithSystemPrompt("You are a helpful math tutor. Always explain your answer."),
		gemini.WithMaxTokens(100),
		gemini.WithTemperature(0.5),
	)
	if err != nil {
		if strings.Contains(err.Error(), "429") || strings.Contains(err.Error(), "quota") {
			t.Skip("Skipping due to rate limiting")
		}
		assert.NoError(err)
		return
	}
	assert.NotNil(response)

	t.Log("Response:", response.Text())
}

func Test_chat_003(t *testing.T) {
	assert := assert.New(t)

	// Test multi-turn conversation
	session := &schema.Session{}
	session.Append(schema.StringMessage("user", "My name is Alice."))

	response, err := client.Chat(context.TODO(), "gemini-2.0-flash", session)
	if err != nil {
		if strings.Contains(err.Error(), "429") || strings.Contains(err.Error(), "quota") {
			t.Skip("Skipping due to rate limiting")
		}
		assert.NoError(err)
		return
	}
	assert.NotNil(response)
	t.Log("Turn 1:", response.Text())

	// Continue the conversation
	session.Append(schema.StringMessage("user", "What is my name?"))
	response, err = client.Chat(context.TODO(), "gemini-2.0-flash", session)
	if err != nil {
		if strings.Contains(err.Error(), "429") || strings.Contains(err.Error(), "quota") {
			t.Skip("Skipping due to rate limiting")
		}
		assert.NoError(err)
		return
	}
	assert.NotNil(response)
	t.Log("Turn 2:", response.Text())

	// The response should mention "Alice"
	assert.Contains(response.Text(), "Alice")
}
