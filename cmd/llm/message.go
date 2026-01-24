package main

import (
	"fmt"
	"os"

	// Packages
	opt "github.com/mutablelogic/go-llm/pkg/opt"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	"github.com/mutablelogic/go-llm/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type MessageCommands struct {
	Ask  AskCommand  `cmd:"" name:"ask" help:"Send a message to a model." group:"MESSAGE"`
	Chat ChatCommand `cmd:"" name:"chat" help:"Chat with a model in a session." group:"MESSAGE"`
}

type AskCommand struct {
	Model string   `arg:"" name:"model" help:"Model name"`
	Text  string   `arg:"" name:"text" help:"Message text to send"`
	File  []string `name:"file" help:"File path(s) to attach (can be used multiple times)" type:"existingfile"`
}

type ChatCommand struct {
	Model string   `arg:"" name:"model" help:"Model name"`
	Text  string   `arg:"" name:"text" help:"Message text to send"`
	File  []string `name:"file" help:"File path(s) to attach (can be used multiple times)" type:"existingfile"`
}

///////////////////////////////////////////////////////////////////////////////
// COMMANDS

func (cmd *AskCommand) Run(ctx *Globals) (err error) {
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
	response, err := agent.WithoutSession(ctx.ctx, *model, message)
	if err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}

	// Print the response
	fmt.Println(response.Text())

	return nil
}

func (cmd *ChatCommand) Run(ctx *Globals) (err error) {
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

	// Build options for session, including toolkit if available
	var sessionopts []opt.Opt
	toolkit, err := ctx.Toolkit()
	if err != nil {
		return err
	}
	if toolkit != nil {
		sessionopts = append(sessionopts, opt.WithToolkit(toolkit))
	}

	// Build options for message
	var msgopts []schema.Opt
	for _, filePath := range cmd.File {
		file, err := os.Open(filePath)
		if err != nil {
			return fmt.Errorf("failed to open file %q: %w", filePath, err)
		}
		defer file.Close()
		msgopts = append(msgopts, schema.WithFile(file))
	}

	// Create message
	message, err := schema.NewMessage(schema.MessageRoleUser, cmd.Text, msgopts...)
	if err != nil {
		return fmt.Errorf("failed to create message: %w", err)
	}

	// Send the message with a new session
	session := make(schema.Session, 0)
	response, err := agent.WithSession(ctx.ctx, *model, types.Ptr(session), message, sessionopts...)
	if err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}

	// Print the response
	fmt.Println(response.Text())

	return nil
}
