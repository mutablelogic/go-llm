package agent

import (
	"context"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Agent represents an AI agent that can process messages and use tools
type Agent interface {
	llm.Client
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

// ListModels returns the list of available models from all clients
func (a *agent) ListModels(ctx context.Context) ([]schema.Model, error) {
	var result []schema.Model
	for _, client := range a.clients {
		models, err := client.ListModels(ctx)
		if err != nil {
			return nil, err
		}
		result = append(result, models...)
	}
	return result, nil
}

// GetModel returns the model with the given name from any client
func (a *agent) GetModel(ctx context.Context, name string) (*schema.Model, error) {
	for _, client := range a.clients {
		model, err := client.GetModel(ctx, name)
		if err == nil && model != nil {
			return model, nil
		}
	}
	return nil, llm.ErrNotFound
}
