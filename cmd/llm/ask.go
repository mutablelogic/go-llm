package main

import (
	"encoding/json"
	"fmt"
	"os"

	// Packages
	otel "github.com/mutablelogic/go-client/pkg/otel"
	httpclient "github.com/mutablelogic/go-llm/pkg/httpclient"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type GenerateCommands struct {
	Ask       AskCommand       `cmd:"" name:"ask" help:"Send a stateless text request to a model." group:"GENERATE"`
	Embedding EmbeddingCommand `cmd:"" name:"embedding" help:"Generate embedding vectors from text." group:"GENERATE"`
}

type AskCommand struct {
	schema.AskRequest
	File string `help:"Path to a file to attach" optional:"" type:"existingfile"`
	URL  string `help:"URL to attach as a reference" optional:""`
}

///////////////////////////////////////////////////////////////////////////////
// COMMANDS

func (cmd *AskCommand) Run(ctx *Globals) (err error) {
	// Load defaults for model and provider when not explicitly set
	if cmd.AskRequest.Model == "" {
		cmd.AskRequest.Model = ctx.defaults.GetString("model")
	}
	if cmd.AskRequest.Provider == "" {
		cmd.AskRequest.Provider = ctx.defaults.GetString("provider")
	}
	if cmd.AskRequest.Model == "" {
		return fmt.Errorf("model is required (set with --model or store a default)")
	}

	// Store model and provider as defaults
	if err := ctx.defaults.Set("model", cmd.AskRequest.Model); err != nil {
		return err
	}
	if cmd.AskRequest.Provider != "" {
		if err := ctx.defaults.Set("provider", cmd.AskRequest.Provider); err != nil {
			return err
		}
	}

	// When format is set, hint the model to reply in JSON
	if len(cmd.AskRequest.Format) > 0 {
		cmd.AskRequest.SystemPrompt = "Respond with valid JSON only. Do not include any other text or formatting.\n" + cmd.AskRequest.SystemPrompt
	}

	client, err := ctx.Client()
	if err != nil {
		return err
	}

	// OTEL
	parent, endSpan := otel.StartSpan(ctx.tracer, ctx.ctx, "AskCommand")
	defer func() { endSpan(err) }()

	// Build options
	var opts []httpclient.AskOpt
	if cmd.File != "" {
		f, err := os.Open(cmd.File)
		if err != nil {
			return err
		}
		defer f.Close()
		opts = append(opts, httpclient.WithFile(f.Name(), f))
	}
	if cmd.URL != "" {
		opts = append(opts, httpclient.WithURL(cmd.URL))
	}

	// Send request
	response, err := client.Ask(parent, cmd.AskRequest, opts...)
	if err != nil {
		return err
	}

	// Print
	if ctx.Debug {
		fmt.Println(response)
	} else {
		// Collect text and thinking from content blocks
		var text, thinking string
		for _, block := range response.Content {
			if block.Text != nil {
				text += *block.Text
			}
			if block.Thinking != nil {
				thinking += *block.Thinking
			}
		}

		// If format was set, try to pretty-print as indented JSON
		if len(cmd.AskRequest.Format) > 0 {
			var raw json.RawMessage
			if err := json.Unmarshal([]byte(text), &raw); err == nil {
				if indented, err := json.MarshalIndent(raw, "", "  "); err == nil {
					fmt.Println(string(indented))
					return nil
				}
			}
		}

		// Print thinking block if present
		if thinking != "" {
			label := "thinking"
			if isTerminal(os.Stdout) {
				label = "\033[2m" + label + "\033[0m" // dim
				thinking = "\033[2m" + thinking + "\033[0m"
			}
			fmt.Println(label + ": " + thinking)
			fmt.Println()
		}

		// Prepend role
		role := response.Role
		if role != "" {
			if isTerminal(os.Stdout) {
				role = "\033[1m" + role + "\033[0m"
			}
			text = role + ": " + text
		}
		fmt.Println(text)
	}
	return nil
}
