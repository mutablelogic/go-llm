package tool

import (
	// Packages
	jsonschema "github.com/google/jsonschema-go/jsonschema"
	llm "github.com/mutablelogic/go-llm"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// DefaultTool provides no-op default implementations of the optional Tool
// interface methods OutputSchema and Meta. Embed it in concrete tool types
// so they satisfy the full Tool interface without boilerplate.
type DefaultTool struct{}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

func (DefaultTool) OutputSchema() (*jsonschema.Schema, error) {
	return nil, nil
}

func (DefaultTool) Meta() llm.ToolMeta {
	return llm.ToolMeta{}
}
