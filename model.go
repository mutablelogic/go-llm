package llm

// An Model can be used to generate a response
type Model interface {
	// Return the name of the model
	Name() string
}
