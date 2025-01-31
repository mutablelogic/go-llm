package llm

import "context"

//////////////////////////////////////////////////////////////////
// TYPES

// Context is fed to the agent to generate a response
type Context interface {
	// Return the role, which can be system, assistant, user, tool, tool_result, ...
	Role() string

	// Return the text of the context
	Text() string

	// Generate a response from a user prompt (with attachments and
	// other empheral options
	FromUser(context.Context, string, ...Opt) (Context, error)

	// Generate a response from a tool, passing the call identifier or
	// function name, and the result
	FromTool(context.Context, string, any, ...Opt) (Context, error)
}
