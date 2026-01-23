package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	// Packages
	tablewriter "github.com/djthorpe/go-tablewriter"
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
	tools := globals.toolkit.Tools(globals.agent.Name())
	fmt.Println(tools)
	return nil
}

func (cmd *ListModelsCmd) Run(globals *Globals) error {
	models, err := globals.agent.ListModels(globals.ctx, cmd.Agent...)
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
}

func (*ListAgentsCmd) Run(globals *Globals) error {
	agents := globals.agent.AgentNames()
	data, err := json.MarshalIndent(agents, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}

func (cmd *DownloadModelCmd) Run(globals *Globals) error {
	agents := globals.agent.AgentsWithName(cmd.Agent)
	if len(agents) == 0 {
		return fmt.Errorf("No agents found with name %q", cmd.Agent)
	}
	switch agents[0].Name() {
	case "ollama":
		model, err := agents[0].(*ollama.Client).PullModel(globals.ctx, cmd.Model, ollama.WithPullStatus(func(status *ollama.PullStatus) {
			var pct int64
			if status.TotalBytes > 0 {
				pct = status.CompletedBytes * 100 / status.TotalBytes
			}
			fmt.Print("\r", status.Status, " ", pct, "%")
			if status.Status == "success" {
				fmt.Println("")
			}
		}))
		if err != nil {
			return err
		}
		fmt.Println(model)
	default:
		return fmt.Errorf("Agent %q does not support model download", agents[0].Name())
	}
	return nil
}

////////////////////////////////////////////////////////////////////////////////
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
