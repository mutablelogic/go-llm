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

type agent struct {
	*opt
}

type model struct {
	Agent     string `json:"agent"`
	llm.Model `json:"model"`
}

var _ llm.Agent = (*agent)(nil)

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// Return a new agent, composed of several different models from different providers
func New(opts ...llm.Opt) (*agent, error) {
	agent := new(agent)
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
func (a *agent) Agents() []string {
	var keys []string
	for k := range a.agents {
		keys = append(keys, k)
	}
	return keys
}

// Return a comma-separated list of agent names
func (a *agent) Name() string {
	return strings.Join(a.Agents(), ",")
}

// Return the models from all agents
func (a *agent) Models(ctx context.Context) ([]llm.Model, error) {
	var result error

	models := make([]llm.Model, 0, 100)
	for _, agent := range a.agents {
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
func (a *agent) Model(ctx context.Context, name string, agent ...string) (llm.Model, error) {
	if len(agent) == 0 {
		agent = a.Agents()
	}

	var result error
	for _, agent := range agent {
		models, err := modelsForAgent(ctx, a.agents[agent], name)
		if err != nil {
			result = errors.Join(result, err)
			continue
		} else if len(models) > 0 {
			return models[0], nil
		}
	}

	// Return any errors
	return nil, result
}

// Generate a response from a prompt
func (a *agent) Generate(context.Context, llm.Model, llm.Context, ...llm.Opt) (*llm.Response, error) {
	return nil, llm.ErrNotImplemented
}

// Embedding vector generation
func (a *agent) Embedding(context.Context, llm.Model, string, ...llm.Opt) ([]float64, error) {
	return nil, llm.ErrNotImplemented
}

// Create user message context
func (a *agent) UserPrompt(string, ...llm.Opt) llm.Context {
	return nil
}

// Create the result of calling a tool
func (a *agent) ToolResult(any) llm.Context {
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
