package agent

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"

	// Packages
	llm "github.com/mutablelogic/go-llm"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type Agent struct {
	*opt
}

type model struct {
	Agent     string `json:"agent"`
	llm.Model `json:"model"`
}

var _ llm.Agent = (*Agent)(nil)

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// Return a new agent, composed of agents and tools
func New(opts ...llm.Opt) (*Agent, error) {
	agent := new(Agent)
	opt, err := apply(opts...)
	if err != nil {
		return nil, err
	} else {
		agent.opt = opt
	}

	// Return success
	return agent, nil
}

///////////////////////////////////////////////////////////////////////////////
// STRINGIFY

func (m model) String() string {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err.Error()
	}
	return string(data)
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Return a list of agent names
func (a *Agent) Agents() []string {
	var keys []string
	for k := range a.agents {
		keys = append(keys, k)
	}
	return keys
}

// Return a list of tool names
func (a *Agent) Tools() []string {
	var keys []string
	for k := range a.tools {
		keys = append(keys, k)
	}
	return keys
}

// Return a comma-separated list of agent names
func (a *Agent) Name() string {
	return strings.Join(a.Agents(), ",")
}

// Return the models from all agents
func (a *Agent) Models(ctx context.Context) ([]llm.Model, error) {
	return a.ListModels(ctx)
}

// Return the models from list of agents
func (a *Agent) ListModels(ctx context.Context, agents ...string) ([]llm.Model, error) {
	var result error

	// Ensure all agents are valid
	for _, agent := range agents {
		if _, exists := a.agents[agent]; !exists {
			result = errors.Join(result, llm.ErrNotFound.Withf("agent %q", agent))
		}
	}

	// Gather models from all agents
	models := make([]llm.Model, 0, 100)
	for _, agent := range a.agents {
		if len(agents) > 0 && !slices.Contains(agents, agent.Name()) {
			continue
		}
		agentmodels, err := modelsForAgent(ctx, agent)
		if err != nil {
			result = errors.Join(result, err)
			continue
		} else {
			models = append(models, agentmodels...)
		}
	}

	// Return the models with any errors
	return models, result
}

// Return a model by name. If no agents are specified, then all agents are considered.
// If multiple agents are specified, then the first model found is returned.
func (a *Agent) GetModel(ctx context.Context, name string, agents ...string) (llm.Model, error) {
	if len(agents) == 0 {
		agents = a.Agents()
	}

	// Ensure all agents are valid
	var result error
	for _, agent := range agents {
		if _, exists := a.agents[agent]; !exists {
			result = errors.Join(result, llm.ErrNotFound.Withf("agent %q", agent))
		}
	}

	// Gather models from agents
	for _, agent := range agents {
		models, err := modelsForAgent(ctx, a.agents[agent], name)
		if err != nil {
			result = errors.Join(result, err)
			continue
		} else if len(models) > 0 {
			return models[0], result
		}
	}

	// Return not found
	result = errors.Join(result, llm.ErrNotFound.Withf("model %q", name))

	// Return any errors
	return nil, result
}

// Embedding vector generation
func (a *Agent) Embedding(context.Context, llm.Model, string, ...llm.Opt) ([]float64, error) {
	return nil, llm.ErrNotImplemented
}

// Create the result of calling a tool
func (a *Agent) ToolResult(id string, opts ...llm.Opt) llm.Context {
	return nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

func modelsForAgent(ctx context.Context, agent llm.Agent, names ...string) ([]llm.Model, error) {
	// Gather models
	models, err := agent.Models(ctx)
	if err != nil {
		return nil, err
	}

	// Filter models
	result := make([]llm.Model, 0, len(models))
	for _, agentmodel := range models {
		if len(names) > 0 && !slices.Contains(names, agentmodel.Name()) {
			continue
		}
		result = append(result, &model{Agent: agent.Name(), Model: agentmodel})
	}

	// Return success
	return result, nil
}
