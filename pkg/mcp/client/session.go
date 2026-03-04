package httpclient

import (
	"context"

	// Packages
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// ServerInfo returns the name, version and negotiated protocol version of the
// connected MCP server. It returns empty strings if the client is not connected.
func (c *Client) ServerInfo() (name, version, protocol string) {
	c.mu.Lock()
	session := c.session
	c.mu.Unlock()
	if session == nil {
		return
	}
	if res := session.InitializeResult(); res != nil {
		if res.ServerInfo != nil {
			name = res.ServerInfo.Name
			version = res.ServerInfo.Version
		}
		protocol = res.ProtocolVersion
	}
	return
}

// ListTools returns all tools advertised by the connected MCP server.
func (c *Client) ListTools(ctx context.Context) ([]*sdkmcp.Tool, error) {
	c.mu.Lock()
	session := c.session
	c.mu.Unlock()
	if session == nil {
		return nil, context.Canceled
	}
	result, err := session.ListTools(ctx, nil)
	if err != nil {
		return nil, err
	}
	return result.Tools, nil
}
