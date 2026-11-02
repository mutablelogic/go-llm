package llm

import (
	"context"
	"log/slog"

	// Packages
	mcp "github.com/modelcontextprotocol/go-sdk/mcp"
	trace "go.opentelemetry.io/otel/trace"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Connector is the interface a remote tool source must implement.
// pkg/mcp/client.Client satisfies this interface.
type Connector interface {
	// Run establishes and drives the connection until ctx is cancelled
	// or the remote server closes it.
	Run(ctx context.Context) error

	// ListTools returns all tools advertised by the connected remote server.
	ListTools(ctx context.Context) ([]Tool, error)

	// ListPrompts returns all prompts advertised by the connected remote server.
	ListPrompts(ctx context.Context) ([]Prompt, error)

	// ListResources returns all resources advertised by the connected remote server.
	ListResources(ctx context.Context) ([]Resource, error)
}

// ConnectorSession is available inside every tool call via SessionFromContext. It
// provides logging and progress reporting back to the connected MCP client,
// as well as read-only metadata about the client.
type ConnectorSession interface {
	// ID returns the unique identifier for this client session.
	ID() string

	// ClientInfo returns the name and version of the connected client, as
	// reported during the MCP handshake. May return nil if unavailable.
	ClientInfo() *mcp.Implementation

	// Capabilities returns the capabilities advertised by the client during
	// the MCP handshake. May return nil if unavailable.
	Capabilities() *mcp.ClientCapabilities

	// Meta returns the _meta map sent by the client in this tool call.
	// Returns nil when no _meta was provided.
	Meta() map[string]any

	// Logger returns a slog.Logger whose output is forwarded to the client
	// as MCP notifications/message events.
	Logger() *slog.Logger

	// Progress sends a progress notification to the client.
	// progress is the amount completed so far; total is the total expected
	// (0 means unknown); message is an optional human-readable status string.
	Progress(progress, total float64, message string) error

	// Tracer returns the OpenTelemetry tracer for distributed tracing.
	// May return nil if no tracer was configured.
	Tracer() trace.Tracer
}

// Prompt is the interface a prompt template advertised by a remote server must implement.
type Prompt interface {
	// Name is the unique identifier of the prompt.
	Name() string

	// Title is a human-readable display name.
	Title() string

	// Description is a human-readable description of the prompt.
	Description() string

	// Prepare returns the prompt for execution, given the input arguments as JSON.
	Prepare(context.Context, ...Resource) (string, []Opt, error)
}
