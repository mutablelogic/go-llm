package main

import (
	"fmt"

	// Packages
	otel "github.com/mutablelogic/go-client/pkg/otel"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type ModelCommands struct {
	ListModels ListModelsCommand `cmd:"" name:"models" help:"List available models." group:"MODEL"`
	GetModel   GetModelCommand   `cmd:"" name:"model" help:"Get model information." group:"MODEL"`
}

type ListModelsCommand struct{}

type GetModelCommand struct {
	Name string `arg:"" name:"name" help:"Model name"`
}

///////////////////////////////////////////////////////////////////////////////
// COMMANDS

func (cmd *ListModelsCommand) Run(ctx *Globals) (err error) {
	client, err := ctx.Client()
	if err != nil {
		return err
	}

	// OTEL tracing
	parent, endSpan := otel.StartSpan(ctx.tracer, ctx.ctx, "ListModelsCommand")
	defer func() { endSpan(err) }()

	// List models
	models, err := client.ListModels(parent)
	if err != nil {
		return err
	}

	// Print each model
	for _, model := range models {
		fmt.Printf("%-40s %s\n", model.Name, model.Description)
	}

	return nil
}

func (cmd *GetModelCommand) Run(ctx *Globals) (err error) {
	client, err := ctx.Client()
	if err != nil {
		return err
	}

	// OTEL tracing
	parent, endSpan := otel.StartSpan(ctx.tracer, ctx.ctx, "GetModelCommand")
	defer func() { endSpan(err) }()

	// Get model
	model, err := client.GetModel(parent, cmd.Name)
	if err != nil {
		return err
	}

	// Print model details
	fmt.Printf("Name:        %s\n", model.Name)
	fmt.Printf("Description: %s\n", model.Description)
	fmt.Printf("Owner:       %s\n", model.OwnedBy)

	return nil
}
