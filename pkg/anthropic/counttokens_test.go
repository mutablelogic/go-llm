package anthropic_test

import (
	"context"
	"testing"

	// Packages
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

func Test_counttokens_001(t *testing.T) {
	assert := assert.New(t)

	session := new(schema.Session)
	session.Append(schema.StringMessage("user", "Why is the sky blue?"))

	tokens, err := client.CountTokens(context.TODO(), "claude-haiku-4-5-20251001", session)
	require.NoError(t, err)
	assert.NotZero(tokens)

	t.Logf("Token count: %d", tokens)
}

func Test_counttokens_002(t *testing.T) {
	assert := assert.New(t)

	session := new(schema.Session)
	session.Append(schema.StringMessage("user", "Hello"))
	session.Append(schema.StringMessage("assistant", "Hi there!"))
	session.Append(schema.StringMessage("user", "How are you?"))

	tokens, err := client.CountTokens(context.TODO(), "claude-haiku-4-5-20251001", session)
	require.NoError(t, err)
	assert.NotZero(tokens)

	t.Logf("Token count for multi-turn conversation: %d", tokens)
}
