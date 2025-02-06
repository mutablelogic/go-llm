package llm

import (
	"context"
)

// An Model can be used to generate a response to a user prompt,
// which is passed to an agent. The interaction occurs through
// a session context object.
type Model interface {
	// Return the name of the model
	Name() string

	// Return am empty session context object for the model,
	// setting session options
	Context(...Opt) Context

	// Create a completion from a text prompt
	Completion(context.Context, string, ...Opt) (Completion, error)

	// Create a completion from a chat session
	Chat(context.Context, []Completion, ...Opt) (Completion, error)

	// Embedding vector generation
	Embedding(context.Context, string, ...Opt) ([]float64, error)
}
