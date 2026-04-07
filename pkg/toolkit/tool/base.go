package tool

import (
	// Packages
	llm "github.com/mutablelogic/go-llm"
	jsonschema "github.com/mutablelogic/go-server/pkg/jsonschema"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Base provides default no-op implementations of the optional llm.Tool
// methods. Embed Base in your tool struct to avoid boilerplate; then only
// override the methods you need.
type Base struct{}

///////////////////////////////////////////////////////////////////////////////
// llm.Tool OPTIONAL METHODS

// OutputSchema returns nil, indicating no structured output schema.
func (Base) OutputSchema() *jsonschema.Schema { return nil }

// Meta returns a zero-value ToolMeta (no hints set).
func (Base) Meta() llm.ToolMeta { return llm.ToolMeta{} }
