package client

import (
	"context"
	"encoding/json"

	// Packages
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	tool "github.com/mutablelogic/go-llm/pkg/tool"
)

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// getSession returns the current session under the lock, or an error if not
// connected. Callers hold no lock after this returns, so concurrent calls are safe.
func (c *Client) getSession() (*sdkmcp.ClientSession, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.session == nil {
		return nil, ErrNotConnected
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

// ListTools returns all tools advertised by the connected MCP server as
// tool.Tool values. Each returned tool's Run method invokes CallTool on the
// server. All pages are fetched automatically via the cursor. Returns
// ErrNotConnected if no session is active.
func (c *Client) ListTools(ctx context.Context) ([]tool.Tool, error) {
	sess, err := c.getSession()
	if err != nil {
		return nil, err
	}
	var tools []tool.Tool
	var cursor string
	for {
		params := &sdkmcp.ListToolsParams{Cursor: cursor}
		result, err := sess.ListTools(ctx, params)
		if err != nil {
			return nil, err
		}
		for _, t := range result.Tools {
			tools = append(tools, &mcpTool{t: t, client: c})
		}
		if result.NextCursor == "" {
			break
		}
		cursor = result.NextCursor
	}
	return tools, nil
}

// CallTool invokes a tool on the connected MCP server by name with the given
// arguments as JSON. Follows the same return convention as pkg/tool.Toolkit.Run:
// tool errors (IsError==true in the MCP result) are returned as a Go error;
// on success the plain value is returned: StructuredContent if present,
// the single content item's value if there is exactly one, or a []any slice
// of item values for multiple content items.
// Returns ErrNotConnected if no session is active.
func (c *Client) CallTool(ctx context.Context, name string, arguments json.RawMessage) (any, error) {
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

	// Call the tool and convert the result to (any, error) using the pkg/tool
	res, err := sess.CallTool(ctx, &sdkmcp.CallToolParams{
		Name:      name,
		Arguments: args,
	})
	if err != nil {
		return nil, err
	}
	return callToolResult(res)
}
