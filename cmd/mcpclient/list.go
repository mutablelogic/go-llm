package main

import (
	"fmt"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type ListCmd struct {
	URL string `arg:"" help:"MCP server URL" required:""`
}

///////////////////////////////////////////////////////////////////////////////
// COMMAND

func (cmd *ListCmd) Run(g *Globals) error {
	c, err := g.Connect(cmd.URL)
	if err != nil {
		return err
	}

	name, version, protocol := c.ServerInfo()
	fmt.Printf("Connected to %s %s (protocol %s)\n", name, version, protocol)

	tools, err := c.ListTools(g.ctx)
	if err != nil {
		return fmt.Errorf("list tools: %w", err)
	}

	fmt.Printf("%d tool(s):\n", len(tools))
	for _, t := range tools {
		fmt.Printf("  %-30s  %s\n", t.Name(), t.Description())
	}
	return nil
}
