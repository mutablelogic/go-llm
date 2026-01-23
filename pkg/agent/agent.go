package agent

import (
	// Packages
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Agent represents an AI agent that can process messages and use tools
type Agent interface {
	llm.Client
	llm.Embedder
	llm.Downloader
	llm.Messenger

	// Clients returns a map of client name to client
	Clients() map[string]llm.Client
}

// agent is the concrete implementation of the Agent interface
type agent struct {
	clients map[string]llm.Client
}

var _ Agent = (*agent)(nil)

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// NewAgent creates a new agent with the given options
func NewAgent(opts ...Opt) (Agent, error) {
	a := &agent{
		clients: make(map[string]llm.Client),
	}
	for _, opt := range opts {
		if err := opt(a); err != nil {
			return nil, err
		}
	}
	return a, nil
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Name returns the name of the agent
func (a *agent) Name() string {
	return "agent"
}

// Clients returns a map of client name to client
func (a *agent) Clients() map[string]llm.Client {
	return a.clients
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// clientForModel returns the client that owns the given model
func (a *agent) clientForModel(model schema.Model) llm.Client {
	if model.OwnedBy != "" {
		if client, ok := a.clients[model.OwnedBy]; ok {
			return client
		}
	}
	return nil
}
