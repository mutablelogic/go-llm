package client

import (
	"context"

	// Packages
	mcp "github.com/mutablelogic/go-llm/pkg/mcp/schema"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Ping sends a ping request to the MCP server and returns an error if
// the server does not respond successfully.
func (c *Client) Ping(ctx context.Context) error {
	if err := c.init(ctx); err != nil {
		return err
	}

	req := mcp.Request{
		Version: mcp.RPCVersion,
		Method:  mcp.MessageTypePing,
		ID:      c.nextId(),
	}

	_, err := c.doRPC(ctx, req)
	return err
}
