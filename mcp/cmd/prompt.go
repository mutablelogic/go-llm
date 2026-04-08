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
	mcpclient "github.com/mutablelogic/go-llm/mcp/client"
	tui "github.com/mutablelogic/go-llm/pkg/tui"
	server "github.com/mutablelogic/go-server"
	attribute "go.opentelemetry.io/otel/attribute"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type PromptCommands struct {
	ListPrompts ListPromptsCommand `cmd:"" name:"prompts" help:"List prompts exposed by an MCP server." group:"MCP"`
	GetPrompt   GetPromptCommand   `cmd:"" name:"prompt" help:"Get a prompt by name from an MCP server." group:"MCP"`
}

type ListPromptsCommand struct {
	URLFlag `embed:""`
}

type GetPromptCommand struct {
	URLFlag `embed:""`
	Name    string `arg:"" name:"name" help:"Prompt name"`
	Input   string `arg:"" name:"input" help:"JSON object of string prompt arguments" optional:""`
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

func (cmd *ListPromptsCommand) Run(ctx server.Cmd) (err error) {
	return WithClient(ctx, cmd.URL, func(parentCtx context.Context, client *mcpclient.Client) error {
		parent, endSpan := otel.StartSpan(ctx.Tracer(), parentCtx, "ListPromptsCommand",
			attribute.String("url", cmd.URL),
		)
		defer func() { endSpan(err) }()

		prompts, err := client.ListPrompts(parent)
		if err != nil {
			return err
		}
		if ctx.IsDebug() {
			for _, prompt := range prompts {
				fmt.Println(prompt)
			}
			return nil
		}
		rows := make([][]string, 0, len(prompts))
		for _, prompt := range prompts {
			rows = append(rows, []string{prompt.Name(), prompt.Title(), prompt.Description()})
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

func (cmd *GetPromptCommand) Run(ctx server.Cmd) (err error) {
	return WithClient(ctx, cmd.URL, func(parentCtx context.Context, client *mcpclient.Client) error {
		arguments, err := cmd.arguments()
		if err != nil {
			return err
		}

		parent, endSpan := otel.StartSpan(ctx.Tracer(), parentCtx, "GetPromptCommand",
			attribute.String("url", cmd.URL),
			attribute.String("name", cmd.Name),
		)
		defer func() { endSpan(err) }()

		result, err := client.GetPrompt(parent, cmd.Name, arguments)
		if err != nil {
			return err
		}
		return writePromptResult(os.Stdout, result)
	})
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

func (cmd GetPromptCommand) arguments() (map[string]string, error) {
	return cmd.argumentsWithInput(os.Stdin, stdinHasData(os.Stdin))
}

func (cmd GetPromptCommand) argumentsWithInput(stdin io.Reader, piped bool) (map[string]string, error) {
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

	var arguments map[string]string
	if err := json.Unmarshal([]byte(input), &arguments); err != nil {
		return nil, fmt.Errorf("input must be a JSON object with string values")
	}
	return arguments, nil
}
