package ollama

import (
	"context"

	// Packages

	llm "github.com/mutablelogic/go-llm"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Generate a response from a prompt
func (ollama *Client) Generate(ctx context.Context, model llm.Model, prompt llm.Context, opts ...llm.Opt) (llm.Context, error) {
	// The prompt should be of type *messages
	// Generate a chat response
	response, err := ollama.Chat(ctx, model.Name(), prompt, opts...)
	if err != nil {
		return nil, err
	}

	// Return the response
	return &session{seq: response.Context}, nil
}
