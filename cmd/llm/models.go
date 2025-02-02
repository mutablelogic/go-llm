package main

import (
	"context"
	"encoding/json"
	"fmt"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	agent "github.com/mutablelogic/go-llm/pkg/agent"
	ollama "github.com/mutablelogic/go-llm/pkg/ollama"
)

////////////////////////////////////////////////////////////////////////////////
// TYPES

type ListModelsCmd struct {
	Agent []string `help:"Only return models from a specific agent"`
}

type ListAgentsCmd struct{}

type ListToolsCmd struct{}

type DownloadModelCmd struct {
	Agent string `arg:"" help:"Agent name"`
	Model string `arg:"" help:"Model name"`
}

////////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

func (cmd *ListToolsCmd) Run(globals *Globals) error {
	return runagent(globals, func(ctx context.Context, client llm.Agent) error {
		tools := globals.toolkit.Tools(client)
		fmt.Println(tools)
		return nil
	})
}

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

		agents := make([]string, 0, len(agent.Agents()))
		for _, agent := range agent.Agents() {
			agents = append(agents, agent.Name())
		}

		data, err := json.MarshalIndent(agents, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))

		return nil
	})
}

func (cmd *DownloadModelCmd) Run(globals *Globals) error {
	return runagent(globals, func(ctx context.Context, client llm.Agent) error {
		agent := getagent(client, cmd.Agent)
		if agent == nil {
			return fmt.Errorf("No agents found with name %q", cmd.Agent)
		}
		// Download the model
		switch agent.Name() {
		case "ollama":
			model, err := agent.(*ollama.Client).PullModel(ctx, cmd.Model)
			if err != nil {
				return err
			}
			fmt.Println(model)
		default:
			return fmt.Errorf("Agent %q does not support model download", agent.Name())
		}
		return nil
	})
}

////////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

func runagent(globals *Globals, fn func(ctx context.Context, agent llm.Agent) error) error {
	return fn(globals.ctx, globals.agent)
}

func getagent(client llm.Agent, name string) llm.Agent {
	agent, ok := client.(*agent.Agent)
	if !ok {
		return nil
	}
	for _, agent := range agent.Agents() {
		if agent.Name() == name {
			return agent
		}
	}
	return nil
}
