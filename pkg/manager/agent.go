package manager

import (
	"context"

	// Packages
	schema "github.com/mutablelogic/go-llm/pkg/schema"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// CreateAgent creates a new agent from the given metadata.
func (m *Manager) CreateAgent(ctx context.Context, meta schema.AgentMeta) (*schema.Agent, error) {
	// Resolve the model to ensure it exists, and fill in the provider if not set
	if meta.Model != "" {
		model, err := m.getModel(ctx, meta.Provider, meta.Model)
		if err != nil {
			return nil, err
		}
		meta.Provider = model.OwnedBy
	}

	// Create the agent and return it
	return m.agentStore.CreateAgent(ctx, meta)
}

// GetAgent retrieves an agent by ID or name.
func (m *Manager) GetAgent(ctx context.Context, id string) (*schema.Agent, error) {
	return m.agentStore.GetAgent(ctx, id)
}

// DeleteAgent deletes an agent by ID or name and returns it.
func (m *Manager) DeleteAgent(ctx context.Context, id string) (*schema.Agent, error) {
	a, err := m.agentStore.GetAgent(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := m.agentStore.DeleteAgent(ctx, id); err != nil {
		return nil, err
	}
	return a, nil
}

// ListAgents returns agents with pagination support.
func (m *Manager) ListAgents(ctx context.Context, req schema.ListAgentRequest) (*schema.ListAgentResponse, error) {
	return m.agentStore.ListAgents(ctx, req)
}

// UpdateAgent updates an agent's metadata and creates a new version.
// If Model or Provider are changed, they are validated against the registered providers first.
func (m *Manager) UpdateAgent(ctx context.Context, id string, meta schema.AgentMeta) (*schema.Agent, error) {
	// If model or provider is being changed, validate
	if meta.Model != "" || meta.Provider != "" {
		model, err := m.getModel(ctx, meta.Provider, meta.Model)
		if err != nil {
			return nil, err
		}
		meta.Model = model.Name
		meta.Provider = model.OwnedBy
	}

	// Delegate to store
	return m.agentStore.UpdateAgent(ctx, id, meta)
}
