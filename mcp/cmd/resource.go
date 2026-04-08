package cmd

import (
	"context"
	"fmt"
	"os"

	// Packages
	otel "github.com/mutablelogic/go-client/pkg/otel"
	mcpclient "github.com/mutablelogic/go-llm/mcp/client"
	tui "github.com/mutablelogic/go-llm/pkg/tui"
	server "github.com/mutablelogic/go-server"
	attribute "go.opentelemetry.io/otel/attribute"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type ResourceCommands struct {
	ListResources ListResourcesCommand `cmd:"" name:"resources" help:"List resources exposed by an MCP server." group:"MCP"`
	GetResource   GetResourceCommand   `cmd:"" name:"resource" help:"Get a resource by URI from an MCP server." group:"MCP"`
}

type ListResourcesCommand struct {
	URLFlag `embed:""`
}

type GetResourceCommand struct {
	URLFlag `embed:""`
	URI     string `arg:"" name:"uri" help:"Resource URI"`
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

func (cmd *ListResourcesCommand) Run(ctx server.Cmd) (err error) {
	return WithClient(ctx, cmd.URL, func(parentCtx context.Context, client *mcpclient.Client) error {
		parent, endSpan := otel.StartSpan(ctx.Tracer(), parentCtx, "ListResourcesCommand",
			attribute.String("url", cmd.URL),
		)
		defer func() { endSpan(err) }()

		resources, err := client.ListResources(parent)
		if err != nil {
			return err
		}
		if ctx.IsDebug() {
			for _, resource := range resources {
				fmt.Println(resource)
			}
			return nil
		}
		rows := make([][]string, 0, len(resources))
		for _, resource := range resources {
			rows = append(rows, []string{resource.URI(), resource.Name(), resource.Type(), resource.Description()})
		}
		return writeTable(
			os.Stdout,
			[]string{"URI", "NAME", "TYPE", "DESCRIPTION"},
			[]int{32, 20, 16, 0},
			rows,
			tui.SetWidth(ctx.IsTerm()),
		)
	})
}

func (cmd *GetResourceCommand) Run(ctx server.Cmd) (err error) {
	return WithClient(ctx, cmd.URL, func(parentCtx context.Context, client *mcpclient.Client) error {
		parent, endSpan := otel.StartSpan(ctx.Tracer(), parentCtx, "GetResourceCommand",
			attribute.String("url", cmd.URL),
			attribute.String("uri", cmd.URI),
		)
		defer func() { endSpan(err) }()

		resource, err := client.GetResource(parent, cmd.URI)
		if err != nil {
			return err
		}
		return writeResource(parent, os.Stdout, resource, tui.SetWidth(ctx.IsTerm()))
	})
}
