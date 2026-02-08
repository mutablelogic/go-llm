package schema

import (
	// Packages
	"github.com/google/jsonschema-go/jsonschema"
	types "github.com/mutablelogic/go-llm/pkg/types"
)

////////////////////////////////////////////////////////////////////////////////
// TYPES

// ToolDefinition represents a provider-agnostic tool definition.
// Providers can reshape this into their required payloads.
type ToolDefinition struct {
	Name        string             `json:"name"`
	Description string             `json:"description,omitempty"`
	InputSchema *jsonschema.Schema `json:"input_schema,omitempty"`
}

////////////////////////////////////////////////////////////////////////////////
// STRINGIFY

func (t ToolDefinition) String() string {
	return types.Stringify(t)
}
