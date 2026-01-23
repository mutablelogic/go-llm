package main

import (
	"fmt"
	"os"

	// Packages
	schema "github.com/mutablelogic/go-llm/pkg/schema"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type SendCommands struct {
	Send SendCommand `cmd:"" name:"send" help:"Send a message to a model." group:"MESSAGE"`
}

type SendCommand struct {
	Model string   `arg:"" name:"model" help:"Model name"`
	Text  string   `arg:"" name:"text" help:"Message text to send"`
	File  []string `name:"file" help:"File path(s) to attach (can be used multiple times)" type:"existingfile"`
}

///////////////////////////////////////////////////////////////////////////////
// COMMANDS

func (cmd *SendCommand) Run(ctx *Globals) (err error) {
	// Get agent
	agent, err := ctx.Agent()
	if err != nil {
		return err
	}

	// Get the model
	model, err := agent.GetModel(ctx.ctx, cmd.Model)
	if err != nil {
		return fmt.Errorf("failed to get model %q: %w", cmd.Model, err)
	}

	// Build options for message
	var opts []schema.Opt

	// Add files if provided
	for _, filePath := range cmd.File {
		file, err := os.Open(filePath)
		if err != nil {
			return fmt.Errorf("failed to open file %q: %w", filePath, err)
		}
		defer file.Close()

		opts = append(opts, schema.WithFile(file))
	}

	// Create message
	message, err := schema.NewMessage(schema.MessageRoleUser, cmd.Text, opts...)
	if err != nil {
		return fmt.Errorf("failed to create message: %w", err)
	}

	// Send the message
	response, err := agent.Send(ctx.ctx, *model, message)
	if err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}

	// Print the response
	fmt.Println(response.Text())

	return nil
}
