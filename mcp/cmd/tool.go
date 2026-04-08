package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	// Packages
	otel "github.com/mutablelogic/go-client/pkg/otel"
	schema "github.com/mutablelogic/go-llm/kernel/schema"
	mcpclient "github.com/mutablelogic/go-llm/mcp/client"
	tui "github.com/mutablelogic/go-llm/pkg/tui"
	server "github.com/mutablelogic/go-server"
	attribute "go.opentelemetry.io/otel/attribute"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type ToolCommands struct {
	ListTools ListToolsCommand `cmd:"" name:"tools" help:"List tools exposed by an MCP server." group:"MCP"`
	CallTool  CallToolCommand  `cmd:"" name:"tool-call" help:"Call a tool exposed by an MCP server." group:"MCP"`
}

type ListToolsCommand struct {
	URLFlag `embed:""`
}

type CallToolCommand struct {
	URLFlag `embed:""`
	Name    string `arg:"" name:"name" help:"Tool name"`
	Input   string `arg:"" name:"input" help:"JSON input payload" optional:""`
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

func (cmd *ListToolsCommand) Run(ctx server.Cmd) (err error) {
	return WithClient(ctx, cmd.URL, func(parentCtx context.Context, client *mcpclient.Client) error {
		parent, endSpan := otel.StartSpan(ctx.Tracer(), parentCtx, "ListToolsCommand",
			attribute.String("url", cmd.URL),
		)
		defer func() { endSpan(err) }()

		tools, err := client.ListTools(parent)
		if err != nil {
			return err
		}
		if ctx.IsDebug() {
			for _, tool := range tools {
				fmt.Println(tool)
			}
			return nil
		}
		rows := make([][]string, 0, len(tools))
		for _, tool := range tools {
			rows = append(rows, []string{tool.Name(), tool.Meta().Title, tool.Description()})
		}
		return writeTable(
			os.Stdout,
			[]string{"NAME", "TITLE", "DESCRIPTION"},
			[]int{24, 24, 0},
			rows,
			tui.SetWidth(ctx.IsTerm()),
		)
	})
}

func (cmd *CallToolCommand) Run(ctx server.Cmd) (err error) {
	return WithClient(ctx, cmd.URL, func(parentCtx context.Context, client *mcpclient.Client) error {
		input, err := cmd.request()
		if err != nil {
			return err
		}

		parent, endSpan := otel.StartSpan(ctx.Tracer(), parentCtx, "CallToolCommand",
			attribute.String("url", cmd.URL),
			attribute.String("name", cmd.Name),
		)
		defer func() { endSpan(err) }()

		result, err := client.CallTool(parent, cmd.Name, input)
		if err != nil {
			return err
		}
		return writeValue(os.Stdout, result, tui.SetWidth(ctx.IsTerm()))
	})
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

func (cmd CallToolCommand) request() (json.RawMessage, error) {
	return cmd.requestWithInput(os.Stdin, stdinHasData(os.Stdin))
}

func (cmd CallToolCommand) requestWithInput(stdin io.Reader, piped bool) (json.RawMessage, error) {
	input := strings.TrimSpace(cmd.Input)
	if input == "" && piped && stdin != nil {
		data, err := io.ReadAll(stdin)
		if err != nil {
			return nil, err
		}
		input = strings.TrimSpace(string(data))
	}
	if input == "" {
		return nil, nil
	}
	raw := json.RawMessage(input)
	if !json.Valid(raw) {
		return nil, fmt.Errorf("input must be valid JSON")
	}
	return raw, nil
}

var _ = schema.ErrBadParameter
