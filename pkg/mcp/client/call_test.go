package client

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	// Packages
	schema "github.com/mutablelogic/go-llm/pkg/mcp/schema"
	assert "github.com/stretchr/testify/assert"
)

func Test_client_007(t *testing.T) {
	if os.Getenv("MCP_TEST") == "" {
		t.Skip("Set MCP_TEST=1 to run integration tests")
	}
	assert := assert.New(t)

	c, err := New(endpointURL, clientInfo)
	assert.NoError(err)

	// CallTool - resolve-library-id
	args, _ := json.Marshal(map[string]any{
		"libraryName": "reactjs",
		"query":       "How to use React hooks",
	})
	result, err := c.CallTool(context.Background(), "resolve-library-id", args)
	assert.NoError(err)
	assert.NotNil(result)
	assert.False(result.Error)
	for _, content := range result.Content {
		t.Logf("content type=%s text=%s", content.Type, content.Text[:min(len(content.Text), 200)])
	}
}

func Test_client_008(t *testing.T) {
	if os.Getenv("MCP_TEST") == "" {
		t.Skip("Set MCP_TEST=1 to run integration tests")
	}
	assert := assert.New(t)

	c, err := New(endpointURL, clientInfo)
	assert.NoError(err)

	// CallTool with invalid tool name should return an error
	_, err = c.CallTool(context.Background(), "nonexistent-tool", nil)
	assert.Error(err)
	t.Log(err)

	// Should be a method not found error
	assert.True(isMethodNotFound(err))
}

func Test_client_009(t *testing.T) {
	if os.Getenv("MCP_TEST") == "" {
		t.Skip("Set MCP_TEST=1 to run integration tests")
	}
	assert := assert.New(t)

	c, err := New(endpointURL, clientInfo)
	assert.NoError(err)

	// CallTool with missing required argument should return an error
	args, _ := json.Marshal(map[string]any{
		"query": "How to use React hooks",
		// missing required "libraryName"
	})
	_, err = c.CallTool(context.Background(), "resolve-library-id", args)
	assert.Error(err)
	t.Log(err)

	// Should be an invalid parameters error
	var rpcErr *schema.Error
	assert.ErrorAs(err, &rpcErr)
	assert.Equal(schema.ErrorCodeInvalidParameters, rpcErr.Code)
}
