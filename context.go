package llm

import "context"

//////////////////////////////////////////////////////////////////
// TYPES

// Completion is the content of the last context message
type Completion interface {
	// Return the number of completions, which is ususally 1 unless
	// WithNumCompletions was used
	Num() int

	// Return the current session role, which can be system, assistant,
	// user, tool, tool_result, ...
	// If this is a completion, the role is usually 'assistant'
	Role() string

	// Return the text for the last completion, with the argument as the
	// completion index (usually 0).
	Text(int) string

	// Return audio for the last completion, with the argument as the
	// completion index (usually 0).
	Audio(int) *Attachment

	// Return the current session tool calls given the completion index.
	// Will return nil if no tool calls were returned.
	ToolCalls(int) []ToolCall
}

// Context is fed to the agent to generate a response
type Context interface {
	Completion

	// Generate a response from a user prompt (with attachments and
	// other options)
	FromUser(context.Context, string, ...Opt) error

	// Generate a response from a tool, passing the results
	// from the tool call
	FromTool(context.Context, ...ToolResult) error
}
