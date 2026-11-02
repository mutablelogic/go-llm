package cmd

import (
	"context"
	"fmt"
	"os"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	httpclient "github.com/mutablelogic/go-llm/registry/httpclient"
	schema "github.com/mutablelogic/go-llm/registry/schema"
	server "github.com/mutablelogic/go-server"
	tui "github.com/mutablelogic/go-server/pkg/tui"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type ModelCommands struct {
	ModelList   ModelListCmd   `cmd:"" name:"models" help:"List the models of the providers." group:"LLM MODELS"`
	ModelGet    ModelGetCmd    `cmd:"" name:"model" help:"Get a model by name." group:"LLM MODELS"`
	ModelLoad   ModelLoadCmd   `cmd:"" name:"model-load" help:"Load a model, with a provider which loads models, such as Ollama, which downloads it when it isn't stored." group:"LLM MODELS"`
	ModelDelete ModelDeleteCmd `cmd:"" name:"model-delete" help:"Delete a model, with a provider which deletes models, such as Ollama." group:"LLM MODELS"`
	ModelUnload ModelUnloadCmd `cmd:"" name:"model-unload" help:"Unload a model from memory, keeping it stored, with a provider which loads models, such as Ollama." group:"LLM MODELS"`
}

type ModelListCmd struct {
	schema.ModelListRequest
}

type ModelGetCmd struct {
	Name string `arg:"" name:"name" help:"Model name, or alias, or provider and model name separated by a colon, such as openai:gpt-4o."`
}

type ModelLoadCmd struct {
	Name     string `arg:"" name:"name" help:"Name of the model to load, such as llama3.2:latest."`
	Provider string `name:"provider" help:"Provider to load the model with, which is needed when more than one provider loads models." optional:""`
	Progress bool   `name:"progress" help:"Show the download's progress, when the output is a terminal." default:"true" negatable:""`
}

type ModelDeleteCmd struct {
	Name string `arg:"" name:"name" help:"Model name, or alias, or provider and model name separated by a colon, such as ollama:gemma:latest."`
}

type ModelUnloadCmd struct {
	Name string `arg:"" name:"name" help:"Model name, or alias, or provider and model name separated by a colon, such as ollama:gemma:latest."`
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

func (cmd *ModelListCmd) Run(ctx server.Cmd) error {
	width := ctx.IsTerm()
	debug := ctx.IsDebug()
	return withClient(ctx, "llm.ModelListCmd", func(ctx context.Context, client *httpclient.Client) error {
		models, err := client.ListModels(ctx, cmd.ModelListRequest)
		if err != nil {
			return err
		}

		// With debugging, the response
		if debug {
			fmt.Println(models)
			return nil
		}

		// Otherwise a table of the models, and a summary
		table := tui.TableFor[*schema.Model](tui.SetWidth(width))
		if _, err := table.Write(os.Stdout, models.Body...); err != nil {
			return err
		}
		summary := tui.TableSummary("models", models.Count, uint(len(models.Body)), 0, nil)
		_, err = summary.Write(os.Stdout)
		return err
	})
}

func (cmd *ModelGetCmd) Run(ctx server.Cmd) error {
	return withClient(ctx, "llm.ModelGetCmd", func(ctx context.Context, client *httpclient.Client) error {
		model, err := client.GetModel(ctx, cmd.Name)
		if err != nil {
			return err
		}
		fmt.Println(model)
		return nil
	})
}

func (cmd *ModelLoadCmd) Run(ctx server.Cmd) error {
	// The progress is shown on one line, redrawn as it changes, so only on
	// a terminal
	width := ctx.IsTerm()
	debug := ctx.IsDebug()
	var progress llm.DownloadProgressFn
	if cmd.Progress && width > 0 {
		widget := tui.Progress(tui.SetWidth(width))
		progress = func(status string, percent float64) {
			fmt.Print("\r")
			_, _ = widget.Write(os.Stdout, status, percent)
		}
	}

	return withClient(ctx, "llm.ModelLoadCmd", func(ctx context.Context, client *httpclient.Client) error {
		model, err := client.LoadModel(ctx, schema.LoadModelRequest{
			Provider: cmd.Provider,
			Name:     cmd.Name,
		}, progress)
		if progress != nil {
			fmt.Println()
		}
		if err != nil {
			return err
		}
		if debug {
			fmt.Println(model)
		} else {
			fmt.Println("Loaded model", model.Canonical)
		}
		return nil
	})
}

func (cmd *ModelDeleteCmd) Run(ctx server.Cmd) error {
	debug := ctx.IsDebug()
	return withClient(ctx, "llm.ModelDeleteCmd", func(ctx context.Context, client *httpclient.Client) error {
		model, err := client.DeleteModel(ctx, cmd.Name)
		if err != nil {
			return err
		}
		if debug {
			fmt.Println(model)
		} else {
			fmt.Println("Deleted model", model.Canonical)
		}
		return nil
	})
}

func (cmd *ModelUnloadCmd) Run(ctx server.Cmd) error {
	debug := ctx.IsDebug()
	return withClient(ctx, "llm.ModelUnloadCmd", func(ctx context.Context, client *httpclient.Client) error {
		model, err := client.UnloadModel(ctx, cmd.Name)
		if err != nil {
			return err
		}
		if debug {
			fmt.Println(model)
		} else {
			fmt.Println("Unloaded model", model.Canonical)
		}
		return nil
	})
}
