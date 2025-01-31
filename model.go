package llm

// An Model can be used to generate a response to a user prompt,
// which is passed to an agent. The interaction occurs through
// a session context object.
type Model interface {
	// Return the name of the model
	Name() string

	// Return am empty session context object for the model,
	// setting session options
	Context(...Opt) (Context, error)

	// Convenience method to create a session context object
	// with a user prompt, which panics on error
	MustUserPrompt(string, ...Opt) Context
}
