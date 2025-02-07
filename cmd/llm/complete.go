package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	// Packages
	llm "github.com/mutablelogic/go-llm"
)

////////////////////////////////////////////////////////////////////////////////
// TYPES

type CompleteCmd struct {
	Model       string   `arg:"" help:"Model name"`
	Prompt      string   `arg:"" optional:"" help:"Prompt"`
	File        []string `type:"file" short:"f" help:"Files to attach"`
	System      string   `flag:"system" help:"Set the system prompt"`
	NoStream    bool     `flag:"no-stream" help:"Do not stream output"`
	Format      string   `flag:"format" enum:"text,markdown,json" default:"text" help:"Output format"`
	Temperature *float64 `flag:"temperature" short:"t"  help:"Temperature for sampling"`
}

////////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

func (cmd *CompleteCmd) Run(globals *Globals) error {
	return run(globals, cmd.Model, func(ctx context.Context, model llm.Model) error {
		var prompt []byte

		// If we are pipeline content in via stdin
		fileInfo, err := os.Stdin.Stat()
		if err != nil {
			return llm.ErrInternalServerError.Withf("Failed to get stdin stat: %v", err)
		}
		if (fileInfo.Mode() & os.ModeCharDevice) == 0 {
			if data, err := io.ReadAll(os.Stdin); err != nil {
				return err
			} else if len(data) > 0 {
				prompt = data
			}
		}

		// Append any further prompt
		if len(cmd.Prompt) > 0 {
			prompt = append(prompt, []byte("\n\n")...)
			prompt = append(prompt, []byte(cmd.Prompt)...)
		}

		opts := cmd.opts()
		if !cmd.NoStream {
			// Add streaming callback
			var buf string
			opts = append(opts, llm.WithStream(func(c llm.Completion) {
				fmt.Print(strings.TrimPrefix(c.Text(0), buf))
				buf = c.Text(0)
			}))
		}

		// Add attachments
		for _, file := range cmd.File {
			f, err := os.Open(file)
			if err != nil {
				return err
			}
			defer f.Close()
			opts = append(opts, llm.WithAttachment(f))
		}

		// Make the completion
		completion, err := model.Completion(ctx, string(prompt), opts...)
		if err != nil {
			return err
		}

		// Print the completion
		if cmd.NoStream {
			fmt.Println(completion.Text(0))
		} else {
			fmt.Println("")
		}

		// Return success
		return nil
	})
}

func (cmd *CompleteCmd) opts() []llm.Opt {
	opts := []llm.Opt{}

	// Set system prompt
	var system []string
	if cmd.Format == "markdown" {
		system = append(system, "Structure your output in markdown format.")
	} else if cmd.Format == "json" {
		system = append(system, "Structure your output in JSON format.")
	}
	if cmd.System != "" {
		system = append(system, cmd.System)
	}
	if len(system) > 0 {
		opts = append(opts, llm.WithSystemPrompt(strings.Join(system, "\n")))
	}

	// Set format
	if cmd.Format == "json" {
		opts = append(opts, llm.WithFormat("json"))
	}

	// Set temperature
	if cmd.Temperature != nil {
		opts = append(opts, llm.WithTemperature(*cmd.Temperature))
	}

	return opts
}
