package llm

// An Model can be used to generate a response
type Model interface {
	// Return the name of the model
	Name() string

	// Create user prompt for a model
	UserPrompt(string, ...Opt) Context

	// Create the result of calling a tool for a model
	ToolResult(id string, opts ...Opt) Context
}
