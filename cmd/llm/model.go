package main

import (
	"fmt"

	// Packages
	otel "github.com/mutablelogic/go-client/pkg/otel"
	httpclient "github.com/mutablelogic/go-llm/pkg/httpclient"
	opt "github.com/mutablelogic/go-llm/pkg/opt"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	uitable "github.com/mutablelogic/go-llm/pkg/ui/table"
	types "github.com/mutablelogic/go-server/pkg/types"
	"go.opentelemetry.io/otel/attribute"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type ModelCommands struct {
	Providers  ProvidersCommand  `cmd:"" name:"providers" help:"List providers." group:"MODEL"`
	ListModels ListModelsCommand `cmd:"" name:"models" help:"List models." group:"MODEL"`
	GetModel   GetModelCommand   `cmd:"" name:"model" help:"Get model." group:"MODEL"`
}

type ProvidersCommand struct{}

type ListModelsCommand struct {
	Provider string `arg:"" name:"provider" help:"Filter by provider name" optional:""`
	Limit    *uint  `name:"limit" help:"Maximum number of models to return" optional:""`
	Offset   uint   `name:"offset" help:"Offset for pagination" default:"0"`
}

type GetModelCommand struct {
	Name     string `arg:"" name:"name" help:"Model name"`
	Provider string `name:"provider" help:"Provider name" optional:""`
	Default  bool   `name:"default" help:"Save as default model" optional:""`
}

///////////////////////////////////////////////////////////////////////////////
// COMMANDS

func (cmd *ProvidersCommand) Run(ctx *Globals) (err error) {
	client, err := ctx.Client()
	if err != nil {
		return err
	}

	// OTEL
	parent, endSpan := otel.StartSpan(ctx.tracer, ctx.ctx, "ProvidersCommand")
	defer func() { endSpan(err) }()

	// List models with limit=0 to get only providers
	zero := uint(0)
	response, err := client.ListModels(parent, httpclient.WithLimit(&zero))
	if err != nil {
		return err
	}

	// Print providers
	if ctx.Debug {
		for _, provider := range response.Provider {
			fmt.Println(provider)
		}
	} else if len(response.Provider) > 0 {
		fmt.Println(uitable.Render(schema.ProviderTable(response.Provider)))
	}
	return nil
}

func (cmd *ListModelsCommand) Run(ctx *Globals) (err error) {
	client, err := ctx.Client()
	if err != nil {
		return err
	}

	// OTEL
	parent, endSpan := otel.StartSpan(ctx.tracer, ctx.ctx, "ListModelsCommand",
		attribute.String("request", types.Stringify(cmd)),
	)
	defer func() { endSpan(err) }()

	// Build options
	opts := []opt.Opt{}
	if cmd.Provider != "" {
		opts = append(opts, httpclient.WithProvider(cmd.Provider))
	}
	if cmd.Limit != nil {
		opts = append(opts, httpclient.WithLimit(cmd.Limit))
	}
	if cmd.Offset > 0 {
		opts = append(opts, httpclient.WithOffset(cmd.Offset))
	}

	// List models
	response, err := client.ListModels(parent, opts...)
	if err != nil {
		return err
	}

	// Print
	if ctx.Debug {
		fmt.Println(response)
	} else {
		if len(response.Body) > 0 {
			fmt.Println(uitable.Render(schema.ModelTable{
				Models:       response.Body,
				CurrentModel: ctx.defaults.GetString("model"),
			}))
		}
		fmt.Println(TableSummary(len(response.Body), int(response.Offset), int(response.Count)))
	}
	return nil
}

func (cmd *GetModelCommand) Run(ctx *Globals) (err error) {
	client, err := ctx.Client()
	if err != nil {
		return err
	}

	// OTEL
	parent, endSpan := otel.StartSpan(ctx.tracer, ctx.ctx, "GetModelCommand",
		attribute.String("request", types.Stringify(cmd)),
	)
	defer func() { endSpan(err) }()

	// Build options
	opts := []opt.Opt{}
	if cmd.Provider != "" {
		opts = append(opts, httpclient.WithProvider(cmd.Provider))
	}

	// Get model
	model, err := client.GetModel(parent, cmd.Name, opts...)
	if err != nil {
		return err
	}

	// Print
	fmt.Println(model)

	// Save as default if requested
	if cmd.Default {
		if err := ctx.defaults.Set("model", model.Name); err != nil {
			return err
		}
		if model.OwnedBy != "" {
			if err := ctx.defaults.Set("provider", model.OwnedBy); err != nil {
				return err
			}
		}
	}
	return nil
}
