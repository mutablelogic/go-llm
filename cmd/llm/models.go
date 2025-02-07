package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	// Packages
	tablewriter "github.com/djthorpe/go-tablewriter"
	llm "github.com/mutablelogic/go-llm"
	agent "github.com/mutablelogic/go-llm/pkg/agent"
	"github.com/mutablelogic/go-llm/pkg/ollama"
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
		agent_, ok := client.(*agent.Agent)
		if !ok {
			return fmt.Errorf("No agents found")
		}
		models, err := agent_.ListModels(ctx, cmd.Agent...)
		if err != nil {
			return err
		}
		result := make(ModelList, 0, len(models))
		for _, model := range models {
			result = append(result, Model{
				Agent:       model.(*agent.Model).Agent,
				Model:       model.Name(),
				Description: model.Description(),
				Aliases:     strings.Join(model.Aliases(), ", "),
			})
		}
		// Sort models by name
		sort.Sort(result)

		// Write out the models
		return tablewriter.New(os.Stdout).Write(result, tablewriter.OptOutputText(), tablewriter.OptHeader())
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

// //////////////////////////////////////////////////////////////////////////////
// MODEL LIST

type Model struct {
	Agent       string `json:"agent"  writer:"Agent,width:10"`
	Model       string `json:"model"  writer:"Model,wrap,width:40"`
	Description string `json:"description" writer:"Description,wrap,width:60"`
	Aliases     string `json:"aliases" writer:"Aliases,wrap,width:30"`
}

type ModelList []Model

func (models ModelList) Len() int {
	return len(models)
}

func (models ModelList) Less(a, b int) bool {
	return strings.Compare(models[a].Model, models[b].Model) < 0
}

func (models ModelList) Swap(a, b int) {
	models[a], models[b] = models[b], models[a]
}
