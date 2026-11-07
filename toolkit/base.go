package toolkit

import (
	"time"

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

// Timeout returns zero, as the tool has the time a tool has by default
func (Base) Timeout() time.Duration {
	return 0
}
