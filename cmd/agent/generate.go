package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	agent "github.com/mutablelogic/go-llm/pkg/agent"
)

////////////////////////////////////////////////////////////////////////////////
// TYPES

type GenerateCmd struct {
	Model    string `arg:"" help:"Model name"`
	NoStream bool   `flag:"nostream" help:"Disable streaming"`
}

////////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

func (cmd *GenerateCmd) Run(globals *Globals) error {
	return runagent(globals, func(ctx context.Context, client llm.Agent) error {
		// Get the model
		a, ok := client.(*agent.Agent)
		if !ok {
			return fmt.Errorf("No agents found")
		}
		model, err := a.GetModel(ctx, cmd.Model)
		if err != nil {
			return err
		}

		// Create a session
		session := model.Context(agent.WithStream(!cmd.NoStream))
		if err != nil {
			return err
		}

		// Continue looping until end of input
		for {
			input, err := globals.term.ReadLine(model.Name() + "> ")
			if errors.Is(err, io.EOF) {
				return nil
			} else if err != nil {
				return err
			}

			// Ignore empty input
			input = strings.TrimSpace(input)
			if input == "" {
				continue
			}

			// Feed input into the model
			response, err := session.FromUser(ctx, input)
			if err != nil {
				return err
			}
			fmt.Println(response.Text())

			// Update session
			session = response
		}
	})
}
