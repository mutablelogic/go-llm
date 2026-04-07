package httpclient

import (
	"context"
	"fmt"
	"strings"

	// Packages
	client "github.com/mutablelogic/go-client"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Embedding generates embedding vectors for the given text inputs.
func (c *Client) Embedding(ctx context.Context, req schema.EmbeddingRequest) (*schema.EmbeddingResponse, error) {
	req.Provider = strings.TrimSpace(req.Provider)
	req.Model = strings.TrimSpace(req.Model)
	if req.Model == "" {
		return nil, fmt.Errorf("model name cannot be empty")
	}
	if len(req.Input) == 0 {
		return nil, fmt.Errorf("input cannot be empty")
	}

	httpReq, err := client.NewJSONRequest(req)
	if err != nil {
		return nil, err
	}

	var response schema.EmbeddingResponse
	if err := c.DoWithContext(ctx, httpReq, &response, client.OptPath("embedding")); err != nil {
		return nil, err
	}

	return &response, nil
}
