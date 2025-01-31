package llm

import "context"

//////////////////////////////////////////////////////////////////
// TYPES

// Context is fed to the agent to generate a response
type Context interface {
	// Generate a response from the context
	Generate(context.Context, Model) (Context, error)

	// Append user prompt (and attachments) to a context
	AppendUserPrompt(string, ...Opt) error

	// Append the result of calling a tool to a context
	AppendToolResult(string, ...Opt) error

	// Return the role, which can be assistant, user, tool, tool_result, ...
	Role() string

	// Return the text of the context
	Text() string
}
