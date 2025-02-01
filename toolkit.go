package llm

import (
	"context"
)

////////////////////////////////////////////////////////////////////////////////
// TYPES

// ToolKit is a collection of tools
type ToolKit interface {
	// Register a tool in the toolkit
	Register(Tool) error

	// Return all the tools
	Tools(Agent) []Tool

	// Run the tool calls in parallel
	// TODO: Return tool results
	Run(context.Context, ...ToolCall) error
}

// Definition of a tool
type Tool interface {
	// The name of the tool
	Name() string

	// The description of the tool
	Description() string

	// Run the tool with a deadline and return the result
	// TODO: Change 'any' to ToolResult
	Run(context.Context) (any, error)
}

// A call-out to a tool
type ToolCall interface {
	// The tool name
	Name() string

	// The tool identifier
	Id() string

	// Decode the calling parameters
	Decode(v any) error
}
