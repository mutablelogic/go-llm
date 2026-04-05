package client

import (
	"context"
	"encoding/json"

	// Packages
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
)

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// getSession returns the current session under the lock, or an error if not
// connected. Callers hold no lock after this returns, so concurrent calls are safe.
func (c *Client) getSession() (*sdkmcp.ClientSession, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.session == nil {
		return nil, schema.ErrServiceUnavailable
	}
	return c.session, nil
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// ServerInfo returns the name, version and negotiated protocol version of the
// connected MCP server. It returns empty strings if the client is not connected.
func (c *Client) ServerInfo() (name, version, protocol string) {
	sess, err := c.getSession()
	if err != nil {
		return
	} else if res := sess.InitializeResult(); res != nil {
		if res.ServerInfo != nil {
			name = res.ServerInfo.Name
			version = res.ServerInfo.Version
		}
		protocol = res.ProtocolVersion
	}
	return
}

// ListTools returns the cached list of tools advertised by the connected
// MCP server. The cache is populated on connect and refreshed automatically
// on each ToolListChanged notification. Returns ErrServiceUnavailable if not active.
func (c *Client) ListTools(_ context.Context) ([]llm.Tool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.session == nil {
		return nil, schema.ErrServiceUnavailable
	}
	return c.tools, nil
}

// refreshTools fetches the full tool list from the server, stores it in the
// cache and invokes onToolListChanged if set.
func (c *Client) refreshTools(ctx context.Context) {
	sess, err := c.getSession()
	if err != nil {
		return
	}
	c.mu.Lock()
	tools := make([]llm.Tool, 0, len(c.tools)+10)
	c.mu.Unlock()
	for cursor := ""; ; {
		result, err := sess.ListTools(ctx, &sdkmcp.ListToolsParams{Cursor: cursor})
		if err != nil {
			return
		}
		for _, t := range result.Tools {
			tools = append(tools, &mcpTool{t: t, client: c})
		}
		if cursor = result.NextCursor; cursor == "" {
			break
		}
	}
	c.mu.Lock()
	c.tools = tools
	fn := c.onToolListChanged
	c.mu.Unlock()
	if fn != nil {
		fn(ctx)
	}
}

// CallTool invokes a tool on the connected MCP server by name with the given
// Returns ErrNotConnected if no session is active.
func (c *Client) CallTool(ctx context.Context, name string, arguments json.RawMessage, meta ...schema.MetaValue) (any, error) {
	sess, err := c.getSession()
	if err != nil {
		return nil, err
	}

	// Pass json.RawMessage as any — it implements json.Marshaler so the SDK
	// serialises it verbatim, preserving any JSON value type.
	var args any
	if len(arguments) > 0 {
		args = arguments
	}

	params := &sdkmcp.CallToolParams{
		Name:      name,
		Arguments: args,
	}
	if len(meta) > 0 {
		params.Meta = make(sdkmcp.Meta, len(meta))
		for _, m := range meta {
			params.Meta[m.Key] = m.Value
		}
	}

	// Call the tool and convert the result to (any, error) using the pkg/tool
	res, err := sess.CallTool(ctx, params)
	if err != nil {
		return nil, err
	}
	return callToolResult(res)
}
