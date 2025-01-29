package llm

//////////////////////////////////////////////////////////////////
// TYPES

// Context is fed to the agent to generate a response. Role can be
// assistant, user, tool, tool_result, ...
type Context interface {
	Role() string
}
