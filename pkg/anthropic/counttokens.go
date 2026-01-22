package anthropic

import (
	"context"

	// Packages
	client "github.com/mutablelogic/go-client"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type countTokensRequest struct {
	Messages *schema.Session `json:"messages"`
	Model    string          `json:"model"`
	System   string          `json:"system,omitempty"`
}

type countTokensResponse struct {
	InputTokens uint `json:"input_tokens"`
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// CountTokens counts the number of tokens in a message, including tools, images,
// and documents, without creating it.
func (anthropic *Client) CountTokens(ctx context.Context, model string, session *schema.Session) (uint, error) {
	// Create a request
	request, err := client.NewJSONRequest(countTokensRequest{
		Messages: session,
		Model:    model,
	})
	if err != nil {
		return 0, err
	}

	// Send the request
	var response countTokensResponse
	if err := anthropic.DoWithContext(ctx, request, &response, client.OptPath("messages/count_tokens")); err != nil {
		return 0, err
	}

	// Return the token count
	return response.InputTokens, nil
}

///////////////////////////////////////////////////////////////////////////////
// STRINGIFY

func (r countTokensResponse) String() string {
	return schema.Stringify(r)
}
