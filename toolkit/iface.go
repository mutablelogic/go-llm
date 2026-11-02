package toolkit

import (
	"log/slog"

	// Packages
	mcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

///////////////////////////////////////////////////////////////////////////////
// INTERFACES

type Session interface {
	// ID returns the unique identifier for this client session.
	ID() string

	// ClientInfo returns the name and version of the connected MCP client.
	// Returns nil when called outside an MCP session (e.g. in unit tests).
	ClientInfo() *mcp.Implementation

	// Capabilities returns the capabilities advertised by the client.
	// Returns nil when called outside an MCP session.
	Capabilities() *mcp.ClientCapabilities

	// Meta returns the _meta map sent by the client in this tool call.
	// Returns nil when no _meta was provided.
	Meta() map[string]any

	// Logger returns a slog.Logger whose output is forwarded to the client
	// as MCP notifications/message events.
	Logger() *slog.Logger

	// Progress sends a progress notification back to the MCP caller.
	// progress is the amount completed so far; total is the total expected
	// (0 means unknown); message is an optional human-readable status string.
	// At most one message may be provided; passing more than one returns an error.
	Progress(progress, total float64, message ...string) error
}
