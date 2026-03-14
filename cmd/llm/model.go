package main

import (
	"fmt"
	"strings"

	// Packages
	otel "github.com/mutablelogic/go-client/pkg/otel"
	httpclient "github.com/mutablelogic/go-llm/pkg/httpclient"
	opt "github.com/mutablelogic/go-llm/pkg/opt"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	uitable "github.com/mutablelogic/go-llm/pkg/ui/table"
	server "github.com/mutablelogic/go-server"
	types "github.com/mutablelogic/go-server/pkg/types"
	attribute "go.opentelemetry.io/otel/attribute"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type ModelCommands struct {
	Providers     ProvidersCommand     `cmd:"" name:"providers" help:"List providers." group:"MODEL"`
	ListModels    ListModelsCommand    `cmd:"" name:"models" help:"List models." group:"MODEL"`
	GetModel      GetModelCommand      `cmd:"" name:"model" help:"Get model." group:"MODEL"`
	DownloadModel DownloadModelCommand `cmd:"" name:"download" help:"Download a model." group:"MODEL"`
	DeleteModel   DeleteModelCommand   `cmd:"" name:"delete-model" help:"Delete a model." group:"MODEL"`
}

type ProvidersCommand struct{}

type ListModelsCommand struct {
	Provider string `arg:"" name:"provider" help:"Filter by provider name" optional:""`
	Limit    *uint  `name:"limit" help:"Maximum number of models to return" optional:""`
	Offset   uint   `name:"offset" help:"Offset for pagination" default:"0"`
}

type GetModelCommand struct {
	Name     string `arg:"" name:"name" help:"Model name" optional:""`
	Provider string `name:"provider" help:"Provider name" optional:""`
	Default  bool   `name:"default" help:"Save as default model" optional:""`
}

type DownloadModelCommand struct {
	Name     string `arg:"" name:"name" help:"Model name to download"`
	Provider string `name:"provider" help:"Provider name" optional:""`
	Progress bool   `name:"progress" help:"Show download progress" default:"true" negatable:""`
}

type DeleteModelCommand struct {
	Name     string `arg:"" name:"name" help:"Model name to delete"`
	Provider string `name:"provider" help:"Provider name" optional:""`
}

///////////////////////////////////////////////////////////////////////////////
// COMMANDS

func (cmd *ProvidersCommand) Run(ctx server.Cmd) (err error) {
	client, err := clientFor(ctx)
	if err != nil {
		return err
	}

	// OTEL
	parent, endSpan := otel.StartSpan(ctx.Tracer(), ctx.Context(), "ProvidersCommand")
	defer func() { endSpan(err) }()

	// List models with limit=0 to get only providers
	zero := uint(0)
	response, err := client.ListModels(parent, httpclient.WithLimit(&zero))
	if err != nil {
		return err
	}

	// Print providers
	if ctx.IsDebug() {
		for _, provider := range response.Provider {
			fmt.Println(provider)
		}
	} else if len(response.Provider) > 0 {
		fmt.Println(uitable.Render(schema.ProviderTable(response.Provider)))
	}
	return nil
}

func (cmd *ListModelsCommand) Run(ctx server.Cmd) (err error) {
	client, err := clientFor(ctx)
	if err != nil {
		return err
	}

	// OTEL
	parent, endSpan := otel.StartSpan(ctx.Tracer(), ctx.Context(), "ListModelsCommand",
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
	if ctx.IsDebug() {
		fmt.Println(response)
	} else {
		if len(response.Body) > 0 {
			fmt.Println(uitable.Render(schema.ModelTable{
				Models:       response.Body,
				CurrentModel: ctx.GetString("model"),
			}))
		}
		fmt.Println(TableSummary(len(response.Body), int(response.Offset), int(response.Count)))
	}
	return nil
}

func (cmd *GetModelCommand) Run(ctx server.Cmd) (err error) {
	// Use default model if no name provided
	name := cmd.Name
	if name == "" {
		name = ctx.GetString("model")
		if name == "" {
			return fmt.Errorf("no model specified and no default model set (use --default to save one)")
		}
		// Also use the saved provider if not overridden
		if cmd.Provider == "" {
			cmd.Provider = ctx.GetString("provider")
		}
	}

	// Create the http client
	client, err := clientFor(ctx)
	if err != nil {
		return err
	}

	// OTEL
	parent, endSpan := otel.StartSpan(ctx.Tracer(), ctx.Context(), "GetModelCommand",
		attribute.String("request", types.Stringify(cmd)),
	)
	defer func() { endSpan(err) }()

	// Build options
	opts := []opt.Opt{}
	if cmd.Provider != "" {
		opts = append(opts, httpclient.WithProvider(cmd.Provider))
	}

	// Get model
	model, err := client.GetModel(parent, name, opts...)
	if err != nil {
		return err
	}

	// Print
	fmt.Println(model)

	// Save as default if requested
	if cmd.Default {
		if err := ctx.Set("model", model.Name); err != nil {
			return err
		}
		if model.OwnedBy != "" {
			if err := ctx.Set("provider", model.OwnedBy); err != nil {
				return err
			}
		}
	}

	// Return success
	return nil
}

func (cmd *DownloadModelCommand) Run(ctx server.Cmd) (err error) {
	client, err := clientFor(ctx)
	if err != nil {
		return err
	}

	// OTEL
	parent, endSpan := otel.StartSpan(ctx.Tracer(), ctx.Context(), "DownloadModelCommand",
		attribute.String("request", types.Stringify(cmd)),
	)
	defer func() { endSpan(err) }()

	// Build options
	opts := []opt.Opt{}
	if cmd.Provider != "" {
		opts = append(opts, httpclient.WithProvider(cmd.Provider))
	}
	const barWidth = 20
	if cmd.Progress {
		opts = append(opts, opt.WithProgress(func(status string, percent float64) {
			if percent > 0 {
				filled := int(percent / 100.0 * barWidth)
				bar := strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)
				fmt.Printf("\r  %-30s [%s] %5.1f%%", status, bar, percent)
			} else {
				fmt.Printf("\r  %-52s", status)
			}
		}))
	}

	// Download model
	model, err := client.DownloadModel(parent, cmd.Name, opts...)
	if cmd.Progress {
		fmt.Println() // newline after progress output
	}
	if err != nil {
		return err
	}

	// Print
	if ctx.IsDebug() {
		fmt.Println(model)
	} else {
		fmt.Printf("Downloaded model: %s\n", model.Name)
	}
	return nil
}

func (cmd *DeleteModelCommand) Run(ctx server.Cmd) (err error) {
	client, err := clientFor(ctx)
	if err != nil {
		return err
	}

	// OTEL
	parent, endSpan := otel.StartSpan(ctx.Tracer(), ctx.Context(), "DeleteModelCommand",
		attribute.String("request", types.Stringify(cmd)),
	)
	defer func() { endSpan(err) }()

	// Build options
	opts := []opt.Opt{}
	if cmd.Provider != "" {
		opts = append(opts, httpclient.WithProvider(cmd.Provider))
	}

	// Delete model
	if err := client.DeleteModel(parent, cmd.Name, opts...); err != nil {
		return err
	}

	fmt.Printf("Deleted model: %s\n", cmd.Name)
	return nil
}
