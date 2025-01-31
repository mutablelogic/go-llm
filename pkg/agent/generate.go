package agent

import (
	"context"
	"log"

	// Packages
	llm "github.com/mutablelogic/go-llm"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Generate a response from a prompt
func (a *Agent) Generate(ctx context.Context, m llm.Model, context llm.Context, opts ...llm.Opt) (llm.Context, error) {
	// Obtain the agent
	var agent llm.Agent
	if model, ok := m.(*model); !ok || model == nil {
		return nil, llm.ErrBadParameter.With("model")
	} else if agent_, exists := a.agents[model.Agent]; !exists {
		return nil, llm.ErrNotFound.Withf("agent %q", model.Agent)
	} else {
		agent = agent_
	}

	// Apply the options
	opts, err := translate(agent, opts...)
	if err != nil {
		return nil, err
	}

	log.Print("agent.Generate =>", context, opts)

	// Call Generate for the agent
	return agent.Generate(ctx, m, context, opts...)
}
