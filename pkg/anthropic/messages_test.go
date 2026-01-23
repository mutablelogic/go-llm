package anthropic_test

import (
	"context"
	"testing"

	// Packages
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

func Test_messages_001(t *testing.T) {
	assert := assert.New(t)

	session := new(schema.Session)
	session.Append(schema.NewMessage("user", "Why is the sky blue?"))

	message, err := client.Messages(context.TODO(), "claude-haiku-4-5-20251001", session)
	require.NoError(t, err)
	assert.NotNil(message)

	// Another message
	session.Append(schema.NewMessage("user", "Blue?"))
	_, err = client.Messages(context.TODO(), "claude-haiku-4-5-20251001", session)
	require.NoError(t, err)

	t.Logf("Session (%d tokens): %s", session.Tokens(), session.String())
}
