package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	// Packages
	httpclient "github.com/mutablelogic/go-llm/toolkit/httpclient"
	schema "github.com/mutablelogic/go-llm/toolkit/schema"
	server "github.com/mutablelogic/go-server"
	tui "github.com/mutablelogic/go-server/pkg/tui"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type ToolCommands struct {
	ToolList ToolListCmd `cmd:"" name:"tools" help:"List the tools and agents which models can call." group:"LLM TOOLS & AGENTS"`
	ToolGet  ToolGetCmd  `cmd:"" name:"tool" help:"Get a tool or agent by name, with the JSON schema of its input." group:"LLM TOOLS & AGENTS"`
	ToolCall ToolCallCmd `cmd:"" name:"call" help:"Call a tool or agent, and print its result." group:"LLM TOOLS & AGENTS"`
}

type ToolListCmd struct {
	schema.ToolListRequest
}

type ToolGetCmd struct {
	Name string `arg:"" name:"name" help:"Name of the tool, such as weatherapi:current, or current when only one namespace has a tool with the name."`
}

type ToolCallCmd struct {
	Name  string `arg:"" name:"name" help:"Name of the tool, such as weatherapi:current, or current when only one namespace has a tool with the name."`
	Input string `arg:"" name:"input" help:"Input to the tool as JSON, such as '{\"query\":\"London\"}'." optional:""`
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

func (cmd *ToolListCmd) Run(ctx server.Cmd) error {
	// Width of the terminal
	width := ctx.IsTerm()
	debug := ctx.IsDebug()

	// Perform the request
	return withToolkit(ctx, "llm.ToolListCmd", func(ctx context.Context, client *httpclient.Client) error {
		tools, err := client.ListTools(ctx, cmd.ToolListRequest)
		if err != nil {
			return err
		}

		// With debugging
		if debug {
			fmt.Println(tools)
			return nil
		}

		// Tools list table
		table := tui.TableFor[*schema.Tool](tui.SetWidth(width))
		if _, err := table.Write(os.Stdout, tools.Body...); err != nil {
			return err
		}

		// Tools list summary
		summary := tui.TableSummary("tools", uint(tools.Count), uint(len(tools.Body)), 0, nil)
		if _, err := summary.Write(os.Stdout); err != nil {
			return err
		}

		return nil
	})
}

func (cmd *ToolGetCmd) Run(ctx server.Cmd) error {
	return withToolkit(ctx, "llm.ToolGetCmd", func(ctx context.Context, client *httpclient.Client) error {
		tool, err := client.GetTool(ctx, cmd.Name)
		if err != nil {
			return err
		}

		fmt.Println(tool)
		return nil
	})
}

func (cmd *ToolCallCmd) Run(ctx server.Cmd) error {
	// The input, if any, must be JSON
	var input json.RawMessage
	if text := strings.TrimSpace(cmd.Input); text != "" {
		if !json.Valid([]byte(text)) {
			return fmt.Errorf("invalid input: must be JSON, such as '{\"query\":\"London\"}'")
		}
		input = json.RawMessage(text)
	}

	// Perform the request
	return withToolkit(ctx, "llm.ToolCallCmd", func(ctx context.Context, client *httpclient.Client) error {
		result, err := client.CallTool(ctx, cmd.Name, input)
		if err != nil {
			return err
		}

		// Print the result as indented JSON, or nothing when the tool has no
		// result
		if result == nil {
			return nil
		}
		var buf bytes.Buffer
		if err := json.Indent(&buf, result, "", "  "); err != nil {
			return err
		}
		fmt.Println(buf.String())
		return nil
	})
}
