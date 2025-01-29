package llm

import (
	"context"
)

////////////////////////////////////////////////////////////////////////////////
// TYPES

// A tool can be called from an LLM
type Tool interface {
	// Return the name of the tool
	Name() string

	// Return the description of the tool
	Description() string

	// Tool parameters
	Params() any

	// Execute the tool with parameters
	Run(context.Context, any) error
}
