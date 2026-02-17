package client

import (
	"context"
	"errors"
	"os"
	"testing"

	// Packages
	schema "github.com/mutablelogic/go-llm/pkg/mcp/schema"
	assert "github.com/stretchr/testify/assert"
)

const (
	endpointURL = "https://mcp.context7.com/mcp"
)

// isMethodNotFound returns true if the error is a JSON-RPC -32601 Method not found error.
func isMethodNotFound(err error) bool {
	var rpcErr *schema.Error
	if errors.As(err, &rpcErr) {
		return rpcErr.Code == schema.ErrorCodeMethodNotFound
	}
	return false
}

var clientInfo = schema.ClientInfo{Name: "go-llm-test", Version: "0.0.0"}

func Test_client_001(t *testing.T) {
	assert := assert.New(t)

	c, err := New(endpointURL, clientInfo)
	assert.NoError(err)
	assert.NotNil(c)
	t.Log(c)
}

func Test_client_002(t *testing.T) {
	// Bad URL
	assert := assert.New(t)

	_, err := New("", clientInfo)
	assert.Error(err)
}

func Test_client_003(t *testing.T) {
	if os.Getenv("MCP_TEST") == "" {
		t.Skip("Set MCP_TEST=1 to run integration tests")
	}
	assert := assert.New(t)

	c, err := New(endpointURL, clientInfo)
	assert.NoError(err)
	assert.NotNil(c)

	// Should not be initialized yet
	assert.False(c.initialized)

	// Ping triggers init
	err = c.Ping(context.Background())
	assert.NoError(err)

	// Should now be initialized
	assert.True(c.initialized)
	assert.NotEmpty(c.server.ServerInfo.Name)
	t.Log("sessionId:", c.sessionId)
	t.Log("server:", c.server.ServerInfo.Name, c.server.ServerInfo.Version)
}

func Test_client_004(t *testing.T) {
	if os.Getenv("MCP_TEST") == "" {
		t.Skip("Set MCP_TEST=1 to run integration tests")
	}
	assert := assert.New(t)

	c, err := New(endpointURL, clientInfo)
	assert.NoError(err)

	// ListTools triggers init
	tools, err := c.ListTools(context.Background())
	assert.NoError(err)
	assert.NotNil(tools)
	assert.True(c.initialized)

	for _, tool := range tools {
		t.Logf("tool: %s - %s", tool.Name, tool.Description)
	}
}

func Test_client_005(t *testing.T) {
	if os.Getenv("MCP_TEST") == "" {
		t.Skip("Set MCP_TEST=1 to run integration tests")
	}
	assert := assert.New(t)

	c, err := New(endpointURL, clientInfo)
	assert.NoError(err)

	// ListPrompts triggers init - server may not support this method
	prompts, err := c.ListPrompts(context.Background())
	if isMethodNotFound(err) {
		t.Log("server does not support prompts/list, skipping")
		return
	}
	assert.NoError(err)
	assert.NotNil(prompts)
	t.Logf("prompts: %d", len(prompts))
}

func Test_client_006(t *testing.T) {
	if os.Getenv("MCP_TEST") == "" {
		t.Skip("Set MCP_TEST=1 to run integration tests")
	}
	assert := assert.New(t)

	c, err := New(endpointURL, clientInfo)
	assert.NoError(err)

	// ListResources triggers init - server may not support this method
	resources, err := c.ListResources(context.Background())
	if isMethodNotFound(err) {
		t.Log("server does not support resources/list, skipping")
		return
	}
	assert.NoError(err)
	assert.NotNil(resources)
	t.Logf("resources: %d", len(resources))
}
