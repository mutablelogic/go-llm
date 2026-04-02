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

// ListModels returns a list of models matching the given request parameters.
func (c *Client) ListModels(ctx context.Context, req schema.ModelListRequest) (*schema.ModelList, error) {
	var response schema.ModelList
	if err := c.DoWithContext(ctx, client.MethodGet, &response, client.OptPath("model"), client.OptQuery(req.Query())); err != nil {
		return nil, err
	}

	// Return success
	return &response, nil
}

// GetModel retrieves a specific model, optionally scoped to a provider.
func (c *Client) GetModel(ctx context.Context, req schema.GetModelRequest) (*schema.Model, error) {
	req.Name = strings.TrimSpace(req.Name)
	req.Provider = strings.TrimSpace(req.Provider)
	if req.Name == "" {
		return nil, fmt.Errorf("model name cannot be empty")
	}

	// Make the request
	requestOpts := make([]client.RequestOpt, 0, 1)
	if req.Provider != "" {
		requestOpts = append(requestOpts, client.OptPath("model", req.Provider, req.Name))
	} else {
		requestOpts = append(requestOpts, client.OptPath("model", req.Name))
	}

	// Make response
	var response schema.Model
	if err := c.DoWithContext(ctx, client.MethodGet, &response, requestOpts...); err != nil {
		return nil, err
	}

	// Return success
	return &response, nil
}
