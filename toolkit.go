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

	// Run the tool calls in parallel and return the results
	Run(context.Context, ...ToolCall) ([]ToolResult, error)
}

// Definition of a tool
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

	// Decode the calling parameters
	Decode(v any) error
}

// Results from calling tools
type ToolResult interface {
	// The call associated with the result
	Call() ToolCall

	// The result, which can be encoded into json
	Value() any
}
