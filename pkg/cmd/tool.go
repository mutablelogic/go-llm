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

type ToolCommands struct {
	ListTools ListToolsCommand `cmd:"" name:"tools" help:"List tools." group:"TOOL"`
	GetTool   GetToolCommand   `cmd:"" name:"tool" help:"Get a tool by name." group:"TOOL"`
}

type ListToolsCommand struct {
	schema.ToolListRequest `embed:""`
}

type GetToolCommand struct {
	Name string `arg:"" name:"name" help:"Tool name"`
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

func (cmd *ListToolsCommand) Run(ctx server.Cmd) (err error) {
	return WithClient(ctx, func(client *httpclient.Client, _ string) error {
		parent, endSpan := otel.StartSpan(ctx.Tracer(), ctx.Context(), "ListToolsCommand",
			attribute.String("request", types.Stringify(cmd.ToolListRequest)),
		)
		defer func() { endSpan(err) }()

		tools, err := client.ListTools(parent, cmd.ToolListRequest)
		if err != nil {
			return err
		}

		if ctx.IsDebug() {
			fmt.Println(tools)
			return nil
		}

		_, err = tui.TableFor[schema.ToolMeta](tui.SetWidth(ctx.IsTerm())).Write(os.Stdout, tools.Body...)
		return err
	})
}

func (cmd *GetToolCommand) Run(ctx server.Cmd) (err error) {
	return WithClient(ctx, func(client *httpclient.Client, _ string) error {
		parent, endSpan := otel.StartSpan(ctx.Tracer(), ctx.Context(), "GetToolCommand",
			attribute.String("name", cmd.Name),
		)
		defer func() { endSpan(err) }()

		tool, err := client.GetTool(parent, cmd.Name)
		if err != nil {
			return err
		}

		fmt.Println(tool)
		return nil
	})
}
