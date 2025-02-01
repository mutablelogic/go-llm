package llm

import "context"

//////////////////////////////////////////////////////////////////
// TYPES

// ContextContent is the content of the last context message
type ContextContent interface {
	// Return the current session role, which can be system, assistant, user, tool, tool_result, ...
	Role() string

	// Return the current session text, or empty string if no text was returned
	Text() string

	// Return the current session tool calls, or empty if no tool calls were made
	ToolCalls() []ToolCall
}

// Context is fed to the agent to generate a response
type Context interface {
	ContextContent

	// Generate a response from a user prompt (with attachments and
	// other empheral options
	FromUser(context.Context, string, ...Opt) error

	// Generate a response from a tool, passing the call identifier or
	// function name, and the result
	FromTool(context.Context, string, any, ...Opt) error
}
