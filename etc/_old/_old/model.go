package llm

import (
	"context"
)

// An Model can be used to generate a response to a user prompt,
// which is passed to an agent. A back-and-forth interaction occurs through
// a session context object.
type Model interface {
	// Return the name of the model
	Name() string

	// Return the description of the model
	Description() string

	// Return any model aliases
	Aliases() []string

	// Return am empty session context object for the model, setting
	// session options
	Context(...Opt) Context

	// Create a completion from a text prompt, including image
	// and audio (TTS) generation
	Completion(context.Context, string, ...Opt) (Completion, error)

	// Embedding vector generation
	Embedding(context.Context, string, ...Opt) ([]float64, error)
}
