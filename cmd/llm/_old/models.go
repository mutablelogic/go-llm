package main

import (
	"fmt"

	// Packages
	otel "github.com/mutablelogic/go-client/pkg/otel"
	agent "github.com/mutablelogic/go-llm/pkg/agent"
	opt "github.com/mutablelogic/go-llm/pkg/opt"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type ModelCommands struct {
	ListModels ListModelsCommand `cmd:"" name:"models" help:"List available models." group:"MODEL"`
	GetModel   GetModelCommand   `cmd:"" name:"model" help:"Get model information." group:"MODEL"`
}

type ListModelsCommand struct {
	Provider string `name:"provider" help:"Filter models by provider name" optional:""`
}

type GetModelCommand struct {
	Name string `arg:"" name:"name" help:"Model name"`
}

///////////////////////////////////////////////////////////////////////////////
// COMMANDS

func (cmd *ListModelsCommand) Run(ctx *Globals) (err error) {
	client, err := ctx.Agent()
	if err != nil {
		return err
	}

	// OTEL tracing
	parent, endSpan := otel.StartSpan(ctx.tracer, ctx.ctx, "ListModelsCommand")
	defer func() { endSpan(err) }()

	// Gather options
	var opts []opt.Opt
	if cmd.Provider != "" {
		opts = append(opts, agent.WithProvider(cmd.Provider))
	}

	// List models
	models, err := client.ListModels(parent, opts...)
	if err != nil {
		return err
	}

	// Print models
	fmt.Println(models)
	return nil
}

func (cmd *GetModelCommand) Run(ctx *Globals) (err error) {
	client, err := ctx.Agent()
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

	fmt.Println(model)
	return nil
}
