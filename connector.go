package llm

import (
	"context"
	"encoding/json"
	"log/slog"

	// Packages
	mcp "github.com/modelcontextprotocol/go-sdk/mcp"
	opt "github.com/mutablelogic/go-llm/pkg/opt"
	jsonschema "github.com/mutablelogic/go-server/pkg/jsonschema"
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
	Prepare(ctx context.Context, input json.RawMessage) (string, []opt.Opt, error)
}

// Resource is the interface a readable resource must implement.
type Resource interface {
	// URI returns the unique identifier of the resource. It must be an absolute
	// URI with a non-empty scheme (e.g. "file:///path/to/file").
	URI() string

	// Name returns a human-readable name for the resource.
	Name() string

	// Description returns an optional description of the resource.
	Description() string

	// Type returns the MIME type of the resource content, or an empty
	// string if unknown.
	Type() string

	// Read returns the raw bytes of the resource content.
	Read(ctx context.Context) ([]byte, error)
}

// Tool is an interface for a callable tool with a name, description,
// input schema, optional output schema, and metadata hints.
type Tool interface {
	// Return the name of the tool
	Name() string

	// Return the description of the tool
	Description() string

	// Return the JSON schema for the tool input parameters.
	InputSchema() *jsonschema.Schema

	// Return the JSON schema for the tool output, or nil if unspecified.
	OutputSchema() *jsonschema.Schema

	// Return optional metadata / hints about the tool.
	Meta() ToolMeta

	// Run the tool with the given input as JSON (may be nil)
	Run(ctx context.Context, input json.RawMessage) (any, error)
}

// ToolMeta holds optional metadata about a tool, sourced from the MCP
// ToolAnnotations and protocol _meta fields. All fields are hints only.
type ToolMeta struct {
	// Title is a human-readable display name (takes precedence over Name).
	Title string

	// ReadOnlyHint indicates the tool does not modify its environment.
	ReadOnlyHint bool

	// DestructiveHint, when non-nil and true, indicates the tool may perform
	// destructive updates. Meaningful only when ReadOnlyHint is false.
	DestructiveHint *bool

	// IdempotentHint indicates repeated identical calls have no additional effect.
	// Meaningful only when ReadOnlyHint is false.
	IdempotentHint bool

	// OpenWorldHint, when non-nil and true, indicates the tool may interact
	// with external entities outside a closed domain (e.g. web search).
	OpenWorldHint *bool
}
