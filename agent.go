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

	// Generate a response from a prompt
	Generate(context.Context, Model, Context, ...Opt) (Context, error)

	// Embedding vector generation
	Embedding(context.Context, Model, string, ...Opt) ([]float64, error)

	// Create user message context
	UserPrompt(string, ...Opt) Context

	// Create the result of calling a tool
	ToolResult(id string, opts ...Opt) Context
}
