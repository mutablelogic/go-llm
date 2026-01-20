package ollama_test

import (
	"context"
	"testing"

	// Packages
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

func Test_chat_001(t *testing.T) {
	assert := assert.New(t)

	session := new(schema.Session)
	session.Append(schema.StringMessage("user", "Why is the sky blue?"))

	message, err := client.Chat(context.TODO(), "mistral:7b", session)
	require.NoError(t, err)
	assert.NotNil(message)

	t.Logf("Response: %s", message)
}
