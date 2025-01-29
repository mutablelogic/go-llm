package main

import (
	"context"
	"fmt"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	agent "github.com/mutablelogic/go-llm/pkg/agent"
)

////////////////////////////////////////////////////////////////////////////////
// TYPES

type ListModelsCmd struct {
	Agent []string `help:"Only return models from a specific agent"`
}

type ListAgentsCmd struct{}

////////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

func (cmd *ListModelsCmd) Run(globals *Globals) error {
	return runagent(globals, func(ctx context.Context, client llm.Agent) error {
		agent, ok := client.(*agent.Agent)
		if !ok {
			return fmt.Errorf("No agents found")
		}
		models, err := agent.ListModels(ctx, cmd.Agent...)
		if err != nil {
			return err
		}
		fmt.Println(models)
		return nil
	})
}

func (*ListAgentsCmd) Run(globals *Globals) error {
	return runagent(globals, func(ctx context.Context, client llm.Agent) error {
		agent, ok := client.(*agent.Agent)
		if !ok {
			return fmt.Errorf("No agents found")
		}
		for _, agent := range agent.Agents() {
			fmt.Println(agent)
		}
		return nil
	})
}

////////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

func runagent(globals *Globals, fn func(ctx context.Context, agent llm.Agent) error) error {
	return fn(globals.ctx, globals.agent)
}
