package toolkit

import (
	"context"

	llm "github.com/mutablelogic/go-llm"
)

// Packages

///////////////////////////////////////////////////////////////////////////////
// TYPES

type toolkit struct {
}

var _ Toolkit = (*toolkit)(nil)

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// NewToolkit creates a new Toolkit with the given options.
func New(opts ...Option) (*toolkit, error) {
	return nil, llm.ErrNotImplemented
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// AddTool registers one or more builtin tools.
func (tk *toolkit) AddTool(...llm.Tool) error {
	return llm.ErrNotImplemented
}

// AddPrompt registers one or more builtin prompts.
// Any type implementing llm.Prompt is accepted, including schema.AgentMeta.
func (tk *toolkit) AddPrompt(...llm.Prompt) error {
	return llm.ErrNotImplemented
}

// AddResource registers one or more builtin resources.
func (tk *toolkit) AddResource(...llm.Resource) error {
	return llm.ErrNotImplemented
}

// RemoveBuiltin removes a previously registered builtin tool by name,
// prompt by name, or resource by URI.
// Returns an error if the identifier matches zero or more than one item.
func (tk *toolkit) RemoveBuiltin(string) error {
	return llm.ErrNotImplemented
}

// AddConnector registers a remote MCP server. The namespace is inferred from
// the server (e.g. the hostname or last path segment of the URL). Safe to call
// before or while Run is active; the connector starts immediately if Run is
// already running.
func (tk *toolkit) AddConnector(string) error {
	return llm.ErrNotImplemented
}

// AddConnectorNS registers a remote MCP server under an explicit namespace.
// Safe to call before or while Run is active; the connector starts immediately
// if Run is already running.
func (tk *toolkit) AddConnectorNS(namespace, url string) error {
	return llm.ErrNotImplemented
}

// RemoveConnector removes a connector by URL. Safe to call before or
// while Run is active; the connector is stopped immediately if running.
func (tk *toolkit) RemoveConnector(string) error {
	return llm.ErrNotImplemented
}

// Run starts all queued connectors and blocks until ctx is cancelled.
// It closes the toolkit and waits for all connectors to finish on return.
func (tk *toolkit) Run(context.Context) error {
	return llm.ErrNotImplemented
}

// Lookup finds a tool, prompt, or resource by name, namespace.name, URI,
// or URI#namespace. Returns nil if nothing matches.
func (tk *toolkit) Lookup(context.Context, string) any {
	return llm.ErrNotImplemented
}

// List returns tools, prompts, and resources matching the request.
func (tk *toolkit) List(context.Context, ListRequest) (*ListResponse, error) {
	return nil, llm.ErrNotImplemented
}

// Call executes a tool or prompt, passing optional resource arguments.
// For tools, resources are made available via the session context.
// For prompts, the first resource supplies template variables and any
// remaining resources are attached to the generated message.
func (tk *toolkit) Call(context.Context, any, ...llm.Resource) (llm.Resource, error) {
	return nil, llm.ErrNotImplemented
}
