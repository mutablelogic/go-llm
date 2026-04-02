package cmd

import (
	"fmt"

	// Packages
	otel "github.com/mutablelogic/go-client/pkg/otel"
	httpclient "github.com/mutablelogic/go-llm/pkg/httpclient-new"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	server "github.com/mutablelogic/go-server"
	types "github.com/mutablelogic/go-server/pkg/types"
	attribute "go.opentelemetry.io/otel/attribute"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type ModelCommands struct {
	ListModels ListModelsCommand `cmd:"" name:"models" help:"List models." group:"PROVIDER MODELS"`
}

type ListModelsCommand struct {
	schema.ModelListRequest `embed:""`
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

		fmt.Println(models)
		return nil
	})
}
