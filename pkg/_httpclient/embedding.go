package httpclient

import (
	"context"
	"fmt"

	// Packages
	client "github.com/mutablelogic/go-client"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Embedding generates embedding vectors for the given text inputs.
func (c *Client) Embedding(ctx context.Context, req schema.EmbeddingRequest) (*schema.EmbeddingResponse, error) {
	if req.Model == "" {
		return nil, fmt.Errorf("model name cannot be empty")
	}
	if len(req.Input) == 0 {
		return nil, fmt.Errorf("input cannot be empty")
	}

	// Create request
	payload, err := client.NewJSONRequest(req)
	if err != nil {
		return nil, err
	}
	reqOpts := []client.RequestOpt{client.OptPath("embedding")}

	// Perform request
	var response schema.EmbeddingResponse
	if err := c.DoWithContext(ctx, payload, &response, reqOpts...); err != nil {
		return nil, err
	}

	// Return the response
	return &response, nil
}
