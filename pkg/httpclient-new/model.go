package httpclient

import (
	"context"

	// Packages
	client "github.com/mutablelogic/go-client"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// ListModels returns a list of models matching the given request parameters.
func (c *Client) ListModels(ctx context.Context, req schema.ModelListRequest) (*schema.ModelList, error) {
	var response schema.ModelList
	if err := c.DoWithContext(ctx, client.MethodGet, &response, client.OptPath("model"), client.OptQuery(req.Query())); err != nil {
		return nil, err
	}

	// Return success
	return &response, nil
}
