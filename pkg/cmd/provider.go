package cmd

import (
	"fmt"
	"os"

	// Packages
	otel "github.com/mutablelogic/go-client/pkg/otel"
	httpclient "github.com/mutablelogic/go-llm/pkg/httpclient-new"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	tui "github.com/mutablelogic/go-llm/pkg/tui"
	server "github.com/mutablelogic/go-server"
	types "github.com/mutablelogic/go-server/pkg/types"
	attribute "go.opentelemetry.io/otel/attribute"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type ProviderCommands struct {
	ListProviders  ListProvidersCommand  `cmd:"" name:"providers" help:"List providers." group:"PROVIDERS"`
	CreateProvider CreateProviderCommand `cmd:"" name:"provider-create" help:"Create a provider." group:"PROVIDERS"`
	DeleteProvider DeleteProviderCommand `cmd:"" name:"provider-delete" help:"Delete a provider by name." group:"PROVIDERS"`
	GetProvider    GetProviderCommand    `cmd:"" name:"provider" help:"Get a provider by name." group:"PROVIDERS"`
	UpdateProvider UpdateProviderCommand `cmd:"" name:"provider-update" help:"Update provider metadata." group:"PROVIDERS"`
}

type ListProvidersCommand struct {
	schema.ProviderListRequest `embed:""`
}

type CreateProviderCommand struct {
	schema.ProviderInsert `embed:""`
}

type GetProviderCommand struct {
	Name string `arg:"" name:"name" help:"Provider name"`
}

type DeleteProviderCommand struct {
	Name string `arg:"" name:"name" help:"Provider name"`
}

type UpdateProviderCommand struct {
	Name                string `arg:"" name:"name" help:"Provider name"`
	schema.ProviderMeta `embed:""`
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

func (cmd *ListProvidersCommand) Run(ctx server.Cmd) (err error) {
	return WithClient(ctx, func(client *httpclient.Client, _ string) error {
		parent, endSpan := otel.StartSpan(ctx.Tracer(), ctx.Context(), "ListProvidersCommand",
			attribute.String("request", types.Stringify(cmd.ProviderListRequest)),
		)
		defer func() { endSpan(err) }()

		providers, err := client.ListProviders(parent, cmd.ProviderListRequest)
		if err != nil {
			return err
		}

		// Debug output
		if ctx.IsDebug() {
			fmt.Println(providers)
			return nil
		}

		// Table output
		_, err = tui.TableFor[schema.Provider]().Write(os.Stdout, providers.Body...)
		return err
	})
}

func (cmd *CreateProviderCommand) Run(ctx server.Cmd) (err error) {
	return WithClient(ctx, func(client *httpclient.Client, _ string) error {
		req, err := cmd.request()
		if err != nil {
			return err
		}

		parent, endSpan := otel.StartSpan(ctx.Tracer(), ctx.Context(), "CreateProviderCommand",
			attribute.String("request", types.Stringify(req)),
		)
		defer func() { endSpan(err) }()

		if req.Provider == "" {
			req.Provider = req.Name
		}

		provider, err := client.CreateProvider(parent, req)
		if err != nil {
			return err
		}

		fmt.Println(provider)
		return nil
	})
}

func (cmd *GetProviderCommand) Run(ctx server.Cmd) (err error) {
	return WithClient(ctx, func(client *httpclient.Client, _ string) error {
		parent, endSpan := otel.StartSpan(ctx.Tracer(), ctx.Context(), "GetProviderCommand",
			attribute.String("name", cmd.Name),
		)
		defer func() { endSpan(err) }()

		provider, err := client.GetProvider(parent, cmd.Name)
		if err != nil {
			return err
		}

		fmt.Println(provider)
		return nil
	})
}

func (cmd *DeleteProviderCommand) Run(ctx server.Cmd) (err error) {
	return WithClient(ctx, func(client *httpclient.Client, _ string) error {
		parent, endSpan := otel.StartSpan(ctx.Tracer(), ctx.Context(), "DeleteProviderCommand",
			attribute.String("name", cmd.Name),
		)
		defer func() { endSpan(err) }()

		provider, err := client.DeleteProvider(parent, cmd.Name)
		if err != nil {
			return err
		}

		fmt.Println(provider)
		return nil
	})
}

func (cmd *UpdateProviderCommand) Run(ctx server.Cmd) (err error) {
	return WithClient(ctx, func(client *httpclient.Client, _ string) error {
		req, err := cmd.request()
		if err != nil {
			return err
		}

		parent, endSpan := otel.StartSpan(ctx.Tracer(), ctx.Context(), "UpdateProviderCommand",
			attribute.String("name", cmd.Name),
			attribute.String("request", types.Stringify(req)),
		)
		defer func() { endSpan(err) }()

		provider, err := client.UpdateProvider(parent, cmd.Name, req)
		if err != nil {
			return err
		}

		fmt.Println(provider)
		return nil
	})
}

func (cmd CreateProviderCommand) request() (schema.ProviderInsert, error) {
	return cmd.ProviderInsert, nil
}

func (cmd UpdateProviderCommand) request() (schema.ProviderMeta, error) {
	return cmd.ProviderMeta, nil
}
