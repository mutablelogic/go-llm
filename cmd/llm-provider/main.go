package main

import (
	"fmt"
	"os"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/kernel/schema"
	provider "github.com/mutablelogic/go-llm/provider/registry"
	server "github.com/mutablelogic/go-server"
	servercmd "github.com/mutablelogic/go-server/pkg/cmd"
	types "github.com/mutablelogic/go-server/pkg/types"
	version "github.com/mutablelogic/go-server/pkg/version"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type CLI struct {
	ProviderList   ProviderListCmd   `cmd:"" name:"list" help:"List available providers." group:"PROVIDER"`
	ProviderModels ProviderModelsCmd `cmd:"" name:"models" help:"List available models." group:"PROVIDER"`
}

type ProviderFlags struct {
	Eliza bool `flag:"" name:"eliza" help:"Run the Eliza provider." negatable:""`
}

type ProviderListCmd struct {
	ProviderFlags
}

type ProviderModelsCmd struct {
	ProviderFlags
}

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

const description = "LLM Provider demonstrates how to register multiple LLM providers."

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

func main() {
	if err := servercmd.Main(CLI{}, description, version.Version()); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(-1)
	}
}

///////////////////////////////////////////////////////////////////////////////
// LIST PROVIDERS

func (c ProviderListCmd) Run(ctx server.Cmd) error {
	return c.With(ctx, func(registry *provider.Registry) error {
		fmt.Println("Available providers:", registry.Count())
		return nil
	})
}

///////////////////////////////////////////////////////////////////////////////
// LIST MODELS

func (c ProviderModelsCmd) Run(ctx server.Cmd) error {
	return c.With(ctx, func(registry *provider.Registry) error {
		models, err := registry.ListModels(ctx.Context(), schema.ListModelsRequest{})
		if err != nil {
			return err
		}
		fmt.Println("Available models:", len(models))
		return nil
	})
}

///////////////////////////////////////////////////////////////////////////////
// CREATE PROVIDERS

func (c ProviderFlags) With(ctx server.Cmd, fn func(*provider.Registry) error) error {
	// Create providers
	_, clientopts, err := ctx.ClientEndpoint()
	if err != nil {
		return err
	}

	// Create a provider registry
	registry := provider.New(clientopts...)
	if err != nil {
		return err
	}

	// Add Eliza
	if c.Eliza {
		registry.Set(types.Ptr(schema.Provider{
			Name:     "eliza",
			Provider: "eliza",
		}), schema.ProviderCredentials{})
	}

	if registry.Count() == 0 {
		return llm.ErrNotFound.With("No providers found. Use --eliza to add the Eliza provider.")
	}

	// Call inner function
	return fn(registry)
}
