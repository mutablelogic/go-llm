package manager

import (
	"context"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	agent "github.com/mutablelogic/go-llm/pkg/agent"
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

// CreateAgentSession resolves an agent by ID or name, validates input against
// the agent's schema, executes the agent's template, and creates a new session
// with merged GeneratorMeta and agent labels.
// If Parent is provided, the parent session's GeneratorMeta is used
// as defaults (agent fields take precedence). The returned response contains
// the session ID, rendered text, and tools, which the caller can pass to Chat.
func (m *Manager) CreateAgentSession(ctx context.Context, id string, request schema.CreateAgentSessionRequest) (*schema.CreateAgentSessionResponse, error) {
	if id == "" {
		return nil, llm.ErrBadParameter.With("agent is required")
	}

	// Resolve the agent definition by ID or name
	agentDef, err := m.agentStore.GetAgent(ctx, id)
	if err != nil {
		return nil, err
	}

	// If a parent session is provided, use its GeneratorMeta as defaults
	var defaults schema.GeneratorMeta
	if request.Parent != "" {
		parent, err := m.sessionStore.GetSession(ctx, request.Parent)
		if err != nil {
			return nil, err
		}
		defaults = parent.GeneratorMeta
	}

	// Prepare: validate input, execute template, merge GeneratorMeta
	result, err := agent.Prepare(agentDef, request.Parent, defaults, request.Input)
	if err != nil {
		return nil, err
	}

	// Create a new session from the prepared metadata
	session, err := m.CreateSession(ctx, result.SessionMeta)
	if err != nil {
		return nil, err
	}

	return &schema.CreateAgentSessionResponse{
		Session: session.ID,
		Text:    result.Text,
		Tools:   result.Tools,
	}, nil
}
