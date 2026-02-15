package main

import (
	"fmt"

	// Packages
	otel "github.com/mutablelogic/go-client/pkg/otel"
	httpclient "github.com/mutablelogic/go-llm/pkg/httpclient"
	opt "github.com/mutablelogic/go-llm/pkg/opt"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type ToolCommands struct {
	ListTools ListToolsCommand `cmd:"" name:"tools" help:"List tools." group:"TOOL"`
	GetTool   GetToolCommand   `cmd:"" name:"tool" help:"Get tool." group:"TOOL"`
}

type ListToolsCommand struct {
	Limit  *uint `name:"limit" help:"Maximum number of tools to return" optional:""`
	Offset uint  `name:"offset" help:"Offset for pagination" default:"0"`
}

type GetToolCommand struct {
	Name string `arg:"" name:"name" help:"Tool name"`
}

///////////////////////////////////////////////////////////////////////////////
// COMMANDS

func (cmd *ListToolsCommand) Run(ctx *Globals) (err error) {
	client, err := ctx.Client()
	if err != nil {
		return err
	}

	// OTEL
	parent, endSpan := otel.StartSpan(ctx.tracer, ctx.ctx, "ListToolsCommand")
	defer func() { endSpan(err) }()

	// Build options
	opts := []opt.Opt{}
	if cmd.Limit != nil {
		opts = append(opts, httpclient.WithLimit(cmd.Limit))
	}
	if cmd.Offset > 0 {
		opts = append(opts, httpclient.WithOffset(cmd.Offset))
	}

	// List tools
	response, err := client.ListTools(parent, opts...)
	if err != nil {
		return err
	}

	// Print
	fmt.Println(response)
	return nil
}

func (cmd *GetToolCommand) Run(ctx *Globals) (err error) {
	client, err := ctx.Client()
	if err != nil {
		return err
	}

	// OTEL
	parent, endSpan := otel.StartSpan(ctx.tracer, ctx.ctx, "GetToolCommand")
	defer func() { endSpan(err) }()

	// Get tool
	tool, err := client.GetTool(parent, cmd.Name)
	if err != nil {
		return err
	}

	// Print
	fmt.Println(tool)
	return nil
}
