package cmd

import (
	"fmt"
	"os"

	// Packages
	otel "github.com/mutablelogic/go-client/pkg/otel"
	httpclient "github.com/mutablelogic/go-llm/pkg/httpclient"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	tui "github.com/mutablelogic/go-llm/pkg/tui"
	server "github.com/mutablelogic/go-server"
	types "github.com/mutablelogic/go-server/pkg/types"
	attribute "go.opentelemetry.io/otel/attribute"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type ModelCommands struct {
	ListModels    ListModelsCommand    `cmd:"" name:"models" help:"List models." group:"MODELS"`
	DownloadModel DownloadModelCommand `cmd:"" name:"model-download" help:"Download a model." group:"MODELS"`
	DeleteModel   DeleteModelCommand   `cmd:"" name:"model-delete" help:"Delete a model by name." group:"MODELS"`
	GetModel      GetModelCommand      `cmd:"" name:"model" help:"Get a model by name." group:"MODELS"`
}

type ListModelsCommand struct {
	schema.ModelListRequest `embed:""`
}

type GetModelCommand struct {
	Name      string `arg:"" name:"name" help:"Model name" optional:""`
	Provider  string `name:"provider" help:"Provider name" optional:""`
	Default   bool   `name:"default" help:"Save as the default model" optional:""`
	Embedding bool   `name:"embedding" help:"Use the embedding model defaults instead of the completion model defaults" optional:""`
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
// PUBLIC METHODS

func (cmd *ListModelsCommand) Run(ctx server.Cmd) (err error) {
	return WithClient(ctx, func(client *httpclient.Client, _ string) error {
		parent, endSpan := otel.StartSpan(ctx.Tracer(), ctx.Context(), "ListModelsCommand",
			attribute.String("request", types.Stringify(cmd.ModelListRequest)),
		)
		defer func() { endSpan(err) }()

		models, err := client.ListModels(parent, cmd.ModelListRequest)
		if err != nil {
			return err
		}

		// Debug output
		if ctx.IsDebug() {
			fmt.Println(models)
			return nil
		}

		// Table output
		_, err = tui.TableFor[schema.Model](tui.SetWidth(ctx.IsTerm())).Write(os.Stdout, models.Body...)
		return err
	})
}

func (cmd *GetModelCommand) Run(ctx server.Cmd) (err error) {
	return WithClient(ctx, func(client *httpclient.Client, _ string) error {
		if cmd.Name == "" {
			modelKey, providerKey := cmd.defaultKeys()
			cmd.Name = ctx.GetString(modelKey)
			if cmd.Provider == "" {
				cmd.Provider = ctx.GetString(providerKey)
			}
			if cmd.Name == "" {
				if cmd.Embedding {
					return fmt.Errorf("no embedding model specified and no default embedding model set")
				}
				return fmt.Errorf("no model specified and no default model set")
			}
		}

		req := schema.GetModelRequest{
			Provider: cmd.Provider,
			Name:     cmd.Name,
		}

		// Otel tracing
		parent, endSpan := otel.StartSpan(ctx.Tracer(), ctx.Context(), "GetModelCommand",
			attribute.String("request", types.Stringify(req)),
		)
		defer func() { endSpan(err) }()

		model, err := client.GetModel(parent, req)
		if err != nil {
			return err
		}

		if cmd.Default {
			modelKey, providerKey := cmd.defaultKeys()
			if err := ctx.Set(modelKey, model.Name); err != nil {
				return err
			}
			if model.OwnedBy != "" {
				if err := ctx.Set(providerKey, model.OwnedBy); err != nil {
					return err
				}
			}
		}

		fmt.Println(model)
		return nil
	})
}

func (cmd GetModelCommand) defaultKeys() (string, string) {
	if cmd.Embedding {
		return "embedding_model", "embedding_provider"
	}
	return "model", "provider"
}

func (cmd *DownloadModelCommand) Run(ctx server.Cmd) (err error) {
	return WithClient(ctx, func(client *httpclient.Client, _ string) error {
		req := schema.DownloadModelRequest{
			Provider: cmd.Provider,
			Name:     cmd.Name,
		}

		parent, endSpan := otel.StartSpan(ctx.Tracer(), ctx.Context(), "DownloadModelCommand",
			attribute.String("request", types.Stringify(req)),
		)
		defer func() { endSpan(err) }()

		var progressFn func(string, float64)
		if cmd.Progress {
			widget := tui.Progress(tui.SetWidth(max(10, min(20, ctx.IsTerm()/3))))
			progressFn = func(status string, percent float64) {
				fmt.Print("\r")
				_, _ = widget.Write(os.Stdout, status, percent)
			}
		}

		model, err := client.DownloadModel(parent, req, progressFn)
		if cmd.Progress {
			fmt.Println()
		}
		if err != nil {
			return err
		}

		if ctx.IsDebug() {
			fmt.Println(model)
		} else {
			fmt.Printf("Downloaded model: %s\n", model.Name)
		}
		return nil
	})
}

func (cmd *DeleteModelCommand) Run(ctx server.Cmd) (err error) {
	return WithClient(ctx, func(client *httpclient.Client, _ string) error {
		req := schema.DeleteModelRequest{
			Provider: cmd.Provider,
			Name:     cmd.Name,
		}

		parent, endSpan := otel.StartSpan(ctx.Tracer(), ctx.Context(), "DeleteModelCommand",
			attribute.String("request", types.Stringify(req)),
		)
		defer func() { endSpan(err) }()

		model, err := client.DeleteModel(parent, req)
		if err != nil {
			return err
		}

		if ctx.IsDebug() {
			fmt.Println(model)
		} else {
			fmt.Printf("Deleted model: %s\n", model.Name)
		}
		return nil
	})
}
