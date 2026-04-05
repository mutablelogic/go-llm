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
	llm "github.com/mutablelogic/go-llm"
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
	ListTools ListToolsCommand `cmd:"" name:"tools" help:"List tools." group:"TOOLS & AGENTS"`
	GetTool   GetToolCommand   `cmd:"" name:"tool" help:"Get a tool by name." group:"TOOLS & AGENTS"`
	CallTool  CallToolCommand  `cmd:"" name:"tool-call" help:"Call a tool by name." group:"TOOLS & AGENTS"`
}

type ListToolsCommand struct {
	schema.ToolListRequest `embed:""`
}

type GetToolCommand struct {
	Name string `arg:"" name:"name" help:"Tool name"`
}

type CallToolCommand struct {
	Name  string `arg:"" name:"name" help:"Tool name"`
	Input string `arg:"" name:"input" help:"JSON input payload" optional:""`
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

func (cmd *CallToolCommand) Run(ctx server.Cmd) (err error) {
	return WithClient(ctx, func(client *httpclient.Client, _ string) error {
		req, err := cmd.request()
		if err != nil {
			return err
		}

		parent, endSpan := otel.StartSpan(ctx.Tracer(), ctx.Context(), "CallToolCommand",
			attribute.String("name", cmd.Name),
			attribute.String("request", req.String()),
		)
		defer func() { endSpan(err) }()

		resource, err := client.CallTool(parent, cmd.Name, req)
		if err != nil {
			return err
		}

		if ctx.IsDebug() && resource != nil {
			fmt.Println(resource)
		}

		return writeToolResource(parent, os.Stdout, resource)
	})
}

func (cmd CallToolCommand) request() (schema.CallToolRequest, error) {
	return cmd.requestWithInput(os.Stdin, stdinHasData(os.Stdin))
}

func (cmd CallToolCommand) requestWithInput(stdin io.Reader, piped bool) (schema.CallToolRequest, error) {
	var req schema.CallToolRequest
	input := strings.TrimSpace(cmd.Input)
	if input == "" && piped && stdin != nil {
		data, err := io.ReadAll(stdin)
		if err != nil {
			return schema.CallToolRequest{}, err
		}
		input = strings.TrimSpace(string(data))
	}
	if input == "" {
		return req, nil
	}
	raw := json.RawMessage(input)
	if !json.Valid(raw) {
		return schema.CallToolRequest{}, fmt.Errorf("input must be valid JSON")
	}
	req.Input = raw
	return req, nil
}

func stdinHasData(stdin *os.File) bool {
	if stdin == nil {
		return false
	}
	info, err := stdin.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice == 0
}

func writeToolResource(ctx context.Context, w io.Writer, resource llm.Resource) error {
	if resource == nil {
		return nil
	}
	data, err := resource.Read(ctx)
	if err != nil {
		return err
	}

	switch {
	case resource.Type() == types.ContentTypeJSON:
		var raw json.RawMessage
		if json.Unmarshal(data, &raw) == nil {
			if indented, err := json.MarshalIndent(raw, "", "  "); err == nil {
				data = append(indented, '\n')
			}
		}
		_, err = w.Write(data)
		return err
	case strings.HasPrefix(resource.Type(), "text/"):
		if _, err := w.Write(data); err != nil {
			return err
		}
		if len(data) == 0 || data[len(data)-1] != '\n' {
			_, err = w.Write([]byte("\n"))
		}
		return err
	default:
		_, err = w.Write(data)
		return err
	}
}
