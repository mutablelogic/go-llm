package main

import (
	"os"
	"strings"

	// Packages
	mcppkg "github.com/mutablelogic/go-llm/pkg/mcp"
	version "github.com/mutablelogic/go-llm/pkg/version"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type MCPCommands struct {
	Server MCPServerCommand `cmd:"" name:"mcp" help:"Start an MCP server." group:"SERVER"`
}

type MCPServerCommand struct {
	// No additional options needed - uses global API keys
}

///////////////////////////////////////////////////////////////////////////////
// COMMANDS

func (cmd *MCPServerCommand) Run(ctx *Globals) error {
	// Create toolkit with tools
	toolkit, err := ctx.Toolkit()
	if err != nil {
		return err
	}

	// Log tools that will be exposed via MCP
	var toolNames []string
	for _, t := range toolkit.Tools() {
		toolNames = append(toolNames, t.Name())
	}
	if len(toolNames) == 0 {
		ctx.log.Print(ctx.ctx, "Starting MCP server with no tools configured")
	} else {
		ctx.log.Print(ctx.ctx, "Starting MCP server with tools:", strings.Join(toolNames, ", "))
	}

	// Create MCP server
	server, err := mcppkg.New("llm", version.GitTag,
		mcppkg.WithToolKit(toolkit),
	)
	if err != nil {
		return err
	}

	// Run the server on stdio
	return server.RunStdio(ctx.ctx, os.Stdin, os.Stdout)
}
