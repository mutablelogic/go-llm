package llm

import (
	"context"
)

////////////////////////////////////////////////////////////////////////////////
// TYPES

type Tool interface {
	// The name of the tool
	Name() string

	// The description of the tool
	Description() string

	// Run the tool with a deadline and return the result
	Run(context.Context) (any, error)
}

// A call-out to a tool
type ToolCall interface {
	// The tool name
	Name() string

	// The tool identifier
	Id() string

	// The calling parameters
	Params() any
}
