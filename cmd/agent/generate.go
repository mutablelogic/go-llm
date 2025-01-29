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

type GenerateCmd struct {
	Model string `arg:"" help:"Model name"`
	Text  string `arg:"" help:"Text to generate a response for"`
}

////////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

func (cmd *GenerateCmd) Run(globals *Globals) error {
	return runagent(globals, func(ctx context.Context, client llm.Agent) error {
		// Get the model
		// TODO: Model should be cached
		agent, ok := client.(*agent.Agent)
		if !ok {
			return fmt.Errorf("No agents found")
		}
		model, err := agent.GetModel(ctx, cmd.Model)
		if err != nil {
			return err
		}

		// Generate the content
		response, err := agent.Generate(ctx, model, agent.UserPrompt(cmd.Text))
		if err != nil {
			return err
		}

		// Print the response
		fmt.Println("RESPONSE=", response)
		return nil
	})
}
