package toolkit

import (
	"context"
	"log/slog"

	// Packages
	mcp "github.com/modelcontextprotocol/go-sdk/mcp"
	llm "github.com/mutablelogic/go-llm"
)

///////////////////////////////////////////////////////////////////////////////
// INTERFACES

// Toolkit aggregates tools, prompts, and resources from builtins, remote MCP
// connectors, and the manager-backed "user" namespace.
type Toolkit interface {
	// AddTool registers one or more builtin tools.
	AddTool(...llm.Tool) error

	// AddPrompt registers one or more builtin prompts.
	// Any type implementing llm.Prompt is accepted, including schema.AgentMeta.
	AddPrompt(...llm.Prompt) error

	// AddResource registers one or more builtin resources.
	AddResource(...llm.Resource) error

	// RemoveBuiltin removes a previously registered builtin tool by name,
	// prompt by name, or resource by URI. Tools are checked before prompts.
	// Returns schema.ErrNotFound if no match exists.
	RemoveBuiltin(string) error

	// AddConnector registers a remote MCP server. The namespace is inferred from
	// the server (e.g. the hostname or last path segment of the URL). Safe to call
	// before or while Run is active; the connector starts immediately if Run is
	// already running.
	AddConnector(string) error

	// AddConnectorNS registers a remote MCP server under an explicit namespace.
	// Safe to call before or while Run is active; the connector starts immediately
	// if Run is already running.
	AddConnectorNS(namespace, url string) error

	// RemoveConnector removes a connector by URL. Safe to call before or
	// while Run is active; the connector is stopped immediately if running.
	RemoveConnector(string) error

	// Run starts all queued connectors and blocks until ctx is cancelled.
	// It closes the toolkit and waits for all connectors to finish on return.
	Run(context.Context) error

	// Lookup finds a tool, prompt, or resource by name, namespace.name, URI,
	// or URI#namespace. Tools take precedence over prompts when both share a name.
	// Returns schema.ErrNotFound if nothing matches.
	Lookup(context.Context, string) (any, error)

	// List returns tools, prompts, and resources matching the request.
	List(context.Context, ListRequest) (*ListResponse, error)

	// Call executes a tool or prompt, passing optional resource arguments.
	// For tools, resources are made available via the session context.
	// For prompts, the first resource supplies template variables and any
	// remaining resources are attached to the generated message.
	Call(context.Context, any, ...llm.Resource) (llm.Resource, error)
}

type ToolkitDelegate interface {
	// OnEvent is called when a lifecycle or list-change notification is fired.
	// ConnectorEventStateChange events are handled internally by the toolkit and
	// are never forwarded here. For all other connector-originated events the
	// Connector field is set to the originating connector; for builtin add/remove
	// operations Connector will be nil.
	OnEvent(ConnectorEvent)

	// Call executes a prompt via the manager, passing optional input resources.
	Call(context.Context, llm.Prompt, ...llm.Resource) (llm.Resource, error)

	// CreateConnector is called to create a new connector for the given URL.
	// The onEvent callback must be called by the connector to report lifecycle
	// and list-change events back to the toolkit. The toolkit injects the
	// Connector field before forwarding to OnEvent, so the caller need not set it.
	CreateConnector(url string, onEvent func(ConnectorEvent)) (llm.Connector, error)
}

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
