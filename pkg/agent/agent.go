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
	*llm.Opts
}

type Model struct {
	Agent     string `json:"agent"`
	llm.Model `json:"model"`
}

var _ llm.Agent = (*Agent)(nil)

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// Return a new agent, composed of a series of agents and tools
func New(opts ...llm.Opt) (*Agent, error) {
	agent := new(Agent)
	if opts, err := llm.ApplyOpts(opts...); err != nil {
		return nil, err
	} else {
		agent.Opts = opts
	}

	// Return success
	return agent, nil
}

///////////////////////////////////////////////////////////////////////////////
// STRINGIFY

func (m Model) String() string {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err.Error()
	}
	return string(data)
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Return a list of tool names
func (a *Agent) ToolNames() []string {
	if a.ToolKit() == nil {
		return nil
	}
	var result []string
	for _, t := range a.ToolKit().Tools(a) {
		result = append(result, t.Name())
	}
	return result
}

// Return a list of agent names
func (a *Agent) AgentNames() []string {
	var result []string
	for _, a := range a.Agents() {
		result = append(result, a.Name())
	}
	return result
}

// Return a list of agents
func (a *Agent) AgentsWithName(name ...string) []llm.Agent {
	all := a.Agents()
	if len(name) == 0 {
		return all
	}
	result := make([]llm.Agent, 0, len(name))
	for _, a := range all {
		if slices.Contains(name, a.Name()) {
			result = append(result, a)
		}
	}
	return result
}

// Return a comma-separated list of agent names
func (a *Agent) Name() string {
	var keys []string
	for _, agent := range a.Agents() {
		keys = append(keys, agent.Name())
	}
	return strings.Join(keys, ",")
}

// Return the models from all agents
func (a *Agent) Models(ctx context.Context) ([]llm.Model, error) {
	return a.ListModels(ctx)
}

// Return a model
func (a *Agent) Model(ctx context.Context, name string) llm.Model {
	model, err := a.GetModel(ctx, name)
	if err != nil {
		panic(err)
	}
	return model
}

// Return the models from list of agents
func (a *Agent) ListModels(ctx context.Context, names ...string) ([]llm.Model, error) {
	var result error

	// Gather models from agents
	agents := a.AgentsWithName(names...)
	models := make([]llm.Model, 0, len(agents)*10)
	for _, agent := range agents {
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
func (a *Agent) GetModel(ctx context.Context, name string, agentnames ...string) (llm.Model, error) {
	var result error

	agents := a.AgentsWithName(agentnames...)
	for _, agent := range agents {
		models, err := modelsForAgent(ctx, agent, name)
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

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

func modelsForAgent(ctx context.Context, agent llm.Agent, names ...string) ([]llm.Model, error) {
	// Gather models
	models, err := agent.Models(ctx)
	if err != nil {
		return nil, err
	}

	match_model := func(model llm.Model, names ...string) bool {
		if len(names) == 0 {
			return true
		}
		if slices.Contains(names, model.Name()) {
			return true
		}
		for _, alias := range model.Aliases() {
			if slices.Contains(names, alias) {
				return true
			}
		}
		return false
	}

	// Filter models
	result := make([]llm.Model, 0, len(models))
	for _, agentmodel := range models {
		if match_model(agentmodel, names...) {
			result = append(result, &Model{Agent: agent.Name(), Model: agentmodel})
		}
	}

	// Return success
	return result, nil
}
