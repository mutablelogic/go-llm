package mcp

import (
	"github.com/mutablelogic/go-llm/pkg/opt"
	"github.com/mutablelogic/go-llm/pkg/tool"
)

/////////////////////////////////////////////////////////////////////////////////
// CONSTANTS

const (
	optToolkit = "mcp.toolkit"
)

/////////////////////////////////////////////////////////////////////////////////
// OPTIONS

// WithToolkit sets the toolkit for the MCP server
func WithToolkit(v *tool.Toolkit) opt.Opt {
	return opt.SetAny(optToolkit, v)
}
