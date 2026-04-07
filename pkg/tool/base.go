package tool

import (
	// Packages
	llm "github.com/mutablelogic/go-llm"
	jsonschema "github.com/mutablelogic/go-server/pkg/jsonschema"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// DefaultTool provides no-op default implementations of the optional Tool
// interface methods OutputSchema and Meta. Embed it in concrete tool types
// so they satisfy the full Tool interface without boilerplate.
type DefaultTool struct{}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

func (DefaultTool) OutputSchema() *jsonschema.Schema { return nil }

func (DefaultTool) Meta() llm.ToolMeta {
	return llm.ToolMeta{}
}
