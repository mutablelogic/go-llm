package toolkit

import (
	"context"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
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
	// prompt by name, or resource by URI.
	// Returns an error if the identifier matches zero or more than one item.
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
	// or URI#namespace. Returns (nil, nil) if nothing matches.
	Lookup(context.Context, string) (any, error)

	// List returns tools, prompts, and resources matching the request.
	List(context.Context, ListRequest) (*ListResponse, error)

	// Call executes a tool or prompt, passing optional resource arguments.
	// For tools, resources are made available via the session context.
	// For prompts, the first resource supplies template variables and any
	// remaining resources are attached to the generated message.
	Call(context.Context, any, ...llm.Resource) (llm.Resource, error)
}

type ToolkitHandler interface {
	// OnStateChange is called when a connector connects or reconnects.
	OnStateChange(llm.Connector, schema.ConnectorState)

	// OnToolListChanged is called when a connector's tool list changes.
	OnToolListChanged(llm.Connector)

	// OnPromptListChanged is called when a connector's prompt list changes.
	OnPromptListChanged(llm.Connector)

	// OnResourceListChanged is called when a connector's resource list changes.
	OnResourceListChanged(llm.Connector)

	// OnResourceUpdated is called when a specific resource (identified by uri) is updated.
	OnResourceUpdated(llm.Connector, string)

	// Call executes a prompt via the manager, passing optional input resources.
	Call(context.Context, llm.Prompt, ...llm.Resource) (llm.Resource, error)

	// List is called to enumerate items in the "user" namespace — prompts and resources
	// stored persistently by the manager (e.g. in a database). Tools are never returned
	// here because they are compiled code, not data.
	List(context.Context, ListRequest) (*ListResponse, error)

	// CreateConnector is called to create a new connector for the given URL.
	// It is called once on AddConnector, and again on each reconnect, so it must return
	// a fresh instance each time (allowing auth tokens to be refreshed).
	CreateConnector(string) (llm.Connector, error)
}
