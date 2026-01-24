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
	ListModels    ListModelsCommand    `cmd:"" name:"models" help:"List available models." group:"MODEL"`
	GetModel      GetModelCommand      `cmd:"" name:"model" help:"Get model information." group:"MODEL"`
	DownloadModel DownloadModelCommand `cmd:"" name:"download" help:"Download a model." group:"MODEL"`
	DeleteModel   DeleteModelCommand   `cmd:"" name:"delete" help:"Delete a model." group:"MODEL"`
}

type ListModelsCommand struct {
	Provider string `name:"provider" help:"Filter models by provider name" optional:""`
}

type GetModelCommand struct {
	Name string `arg:"" name:"name" help:"Model name"`
}

type DownloadModelCommand struct {
	Path string `arg:"" name:"path" help:"Model path in format 'provider:model' (e.g., 'ollama:llama2')"`
}

type DeleteModelCommand struct {
	Name string `arg:"" name:"name" help:"Model name to delete"`
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

func (cmd *DownloadModelCommand) Run(ctx *Globals) (err error) {
	client, err := ctx.Agent()
	if err != nil {
		return err
	}

	// OTEL tracing
	parent, endSpan := otel.StartSpan(ctx.tracer, ctx.ctx, "DownloadModelCommand")
	defer func() { endSpan(err) }()

	// Create progress callback to display download progress
	progressOpt := opt.WithProgress(func(status string, percent float64) {
		// Clear the line and print progress
		fmt.Printf("\r%-50s %6.2f%%", status, percent)
	})

	// Download model with progress tracking
	model, err := client.DownloadModel(parent, cmd.Path, progressOpt)
	if err != nil {
		return err
	}

	// Move to next line after progress completes
	fmt.Println()

	// Print model details
	fmt.Println(model)
	return nil
}

func (cmd *DeleteModelCommand) Run(ctx *Globals) (err error) {
	client, err := ctx.Agent()
	if err != nil {
		return err
	}

	// OTEL tracing
	parent, endSpan := otel.StartSpan(ctx.tracer, ctx.ctx, "DeleteModelCommand")
	defer func() { endSpan(err) }()

	// First get the model to find its owner
	model, err := client.GetModel(parent, cmd.Name)
	if err != nil {
		return err
	}

	// Delete model
	if err := client.DeleteModel(parent, *model); err != nil {
		return err
	}

	// Print model details
	fmt.Println(model)
	return nil
}
