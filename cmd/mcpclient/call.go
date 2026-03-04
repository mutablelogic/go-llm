package main

import (
	"encoding/json"
	"fmt"
	"os"

	// Packages
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type CallCmd struct {
	URL  string `arg:"" help:"MCP server URL" required:""`
	Name string `arg:"" help:"Tool name" required:""`
	Args string `arg:"" help:"JSON arguments object, e.g. '{\"city\":\"London\"}'" optional:""`
}

///////////////////////////////////////////////////////////////////////////////
// COMMAND

func (cmd *CallCmd) Run(g *Globals) error {
	c, err := g.Connect(cmd.URL)
	if err != nil {
		return err
	}

	args := json.RawMessage(cmd.Args)
	if len(args) == 0 {
		args = json.RawMessage("{}")
	}

	result, err := c.CallTool(g.ctx, cmd.Name, args)
	if err != nil {
		return fmt.Errorf("call tool %q: %w", cmd.Name, err)
	}

	// If the result is a plain string that contains valid JSON, unmarshal it
	// first so we pretty-print the inner structure rather than a quoted string.
	if s, ok := result.(string); ok {
		var v any
		if json.Unmarshal([]byte(s), &v) == nil {
			result = v
		}
	}

	_, err = fmt.Fprintln(os.Stdout, types.Stringify(result))
	return err
}
