package ollama_test

import (
	"context"
	"testing"

	// Packages
	ollama "github.com/mutablelogic/go-llm/provider/ollama"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

func Test_Version(t *testing.T) {
	c := requireClient(t)
	version, err := c.Version(context.Background())
	assert.NoError(t, err)
	assert.NotEmpty(t, version)
}

func Test_Version_Unreachable(t *testing.T) {
	c, err := ollama.New("http://localhost:1")
	require.NoError(t, err)

	version, err := c.Version(context.Background())
	assert.Error(t, err)
	assert.Empty(t, version)
}
