package main

import (
	"fmt"

	// Packages
	schema "github.com/mutablelogic/go-llm/pkg/schema"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type SendCommands struct {
	Send SendCommand `cmd:"" name:"send" help:"Send a message to a model." group:"MESSAGE"`
}

type SendCommand struct {
	Model string `arg:"" name:"model" help:"Model name"`
	Text  string `arg:"" name:"text" help:"Message text to send"`
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

	// Create message
	message := schema.StringMessage("user", cmd.Text)

	// Send the message
	response, err := agent.Send(ctx.ctx, *model, &message)
	if err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}

	// Print the response
	fmt.Println(response.Text())

	return nil
}
