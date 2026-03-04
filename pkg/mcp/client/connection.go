package httpclient

import (
	"context"

	// Packages
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Connection describes an active MCP client connection.
// *Client implements this interface.
type Connection interface {
	// Connect establishes the MCP session, auto-detecting the transport.
	// If the server returns 401 and authFn is non-nil, authFn is called and
	// the connection is retried once.
	Connect(ctx context.Context, authFn func(context.Context, string) error) (*sdkmcp.ClientSession, error)

	// Close tears down the background session goroutines and waits for them
	// to exit, ensuring no goroutine leaks.
	Close() error

	// ListTools returns all tools advertised by the server.
	ListTools(ctx context.Context) ([]*sdkmcp.Tool, error)

	// CallTool invokes the named tool with the provided arguments and returns
	// the result.
	CallTool(ctx context.Context, name string, args map[string]any) (*sdkmcp.CallToolResult, error)
}
