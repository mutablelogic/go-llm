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

type ChatCmd struct {
	Model    string `arg:"" help:"Model name"`
	NoStream bool   `flag:"nostream" help:"Disable streaming"`
	System   string `flag:"system" help:"Set the system prompt"`
}

////////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

func (cmd *ChatCmd) Run(globals *Globals) error {
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

		// Set the options
		opts := []llm.Opt{}
		if !cmd.NoStream {
			opts = append(opts, llm.WithStream(func(cc llm.ContextContent) {
				fmt.Println(cc)
			}))
		}
		if cmd.System != "" {
			opts = append(opts, llm.WithSystemPrompt(cmd.System))
		}
		if globals.toolkit != nil {
			opts = append(opts, llm.WithToolKit(globals.toolkit))
		}

		// Create a session
		session := model.Context(opts...)

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
			if err := session.FromUser(ctx, input); err != nil {
				return err
			}

			fmt.Println(session.Text())

			// If there are tool calls, then do these
			calls := session.ToolCalls()
			if results, err := globals.toolkit.Run(ctx, calls...); err != nil {
				return err
			} else {
				fmt.Println(results)
			}
		}
	})
}
