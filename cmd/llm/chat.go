package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	// Packages
	llm "github.com/mutablelogic/go-llm"
)

////////////////////////////////////////////////////////////////////////////////
// TYPES

type ChatCmd struct {
	Model    string `arg:"" help:"Model name"`
	NoStream bool   `flag:"nostream" help:"Disable streaming"`
	NoTools  bool   `flag:"nostream" help:"Disable tool calling"`
	Prompt   string `flag:"prompt" help:"Set the initial user prompt"`
	System   string `flag:"system" help:"Set the system prompt"`
}

////////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

func (cmd *ChatCmd) Run(globals *Globals) error {
	return run(globals, AudioType, cmd.Model, func(ctx context.Context, model llm.Model) error {
		// Current buffer
		var buf string

		// Set the options
		opts := []llm.Opt{}
		if !cmd.NoStream {
			opts = append(opts, llm.WithStream(func(cc llm.Completion) {
				text := cc.Text(0)
				fmt.Print(strings.TrimPrefix(text, buf))
				buf = text
			}))
		}
		if cmd.System != "" {
			opts = append(opts, llm.WithSystemPrompt(cmd.System))
		}
		if globals.toolkit != nil && !cmd.NoTools {
			opts = append(opts, llm.WithToolKit(globals.toolkit))
		}

		// Create a session
		session := model.Context(opts...)

		// Continue looping until end of input
		for {
			var input string
			if cmd.Prompt != "" {
				input = cmd.Prompt
				cmd.Prompt = ""
			} else {
				var err error
				input, err = globals.term.ReadLine(model.Name() + "> ")
				if errors.Is(err, io.EOF) {
					return nil
				} else if err != nil {
					return err
				}
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

			// Repeat call tools until no more calls are made
			for {
				calls := session.ToolCalls(0)
				if len(calls) == 0 {
					break
				}

				if session.Text(0) != "" {
					globals.term.Println(session.Text(0))
				} else {
					var names []string
					for _, call := range calls {
						names = append(names, call.Name())
					}
					globals.term.Println("Calling ", strings.Join(names, ", "))
				}

				if results, err := globals.toolkit.Run(ctx, calls...); err != nil {
					return err
				} else if err := session.FromTool(ctx, results...); err != nil {
					return err
				}
			}

			// Print the response, if not streaming
			if cmd.NoStream {
				globals.term.Println("\n" + session.Text(0) + "\n")
			} else {
				globals.term.Println()
			}
		}
	})
}
