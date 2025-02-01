package llm

import (
	"context"
)

// An LLM Agent is a client for the LLM service
type Agent interface {
	// Return the name of the agent
	Name() string

	// Return the models
	Models(context.Context) ([]Model, error)
}
