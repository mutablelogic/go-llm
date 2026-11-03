package toolkit

import (
	// Packages
	jsonschema "github.com/mutablelogic/go-server/pkg/jsonschema"
)

////////////////////////////////////////////////////////////////////////////////
// TYPES

// Base provides default implementations of the optional llm.Tool methods.
// Embed Base in a tool to avoid boilerplate, and override the methods the
// tool needs.
type Base struct{}

////////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// OutputSchema returns nil, as the tool has no structured output schema
func (Base) OutputSchema() *jsonschema.Schema {
	return nil
}
