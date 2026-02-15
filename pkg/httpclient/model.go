package httpclient

import (
	"context"
	"fmt"

	// Packages
	client "github.com/mutablelogic/go-client"
	opt "github.com/mutablelogic/go-llm/pkg/opt"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// ListModels returns a list of all available models.
// Use WithLimit, WithOffset and WithProvider to paginate and filter results.
func (c *Client) ListModels(ctx context.Context, opts ...opt.Opt) (*schema.ListModelsResponse, error) {
	// Apply options
	o, err := opt.Apply(opts...)
	if err != nil {
		return nil, err
	}

	// Create request
	req := client.NewRequest()
	reqOpts := []client.RequestOpt{client.OptPath("model")}
	if q := o.Query(opt.ProviderKey, opt.LimitKey, opt.OffsetKey); len(q) > 0 {
		reqOpts = append(reqOpts, client.OptQuery(q))
	}

	// Perform request
	var response schema.ListModelsResponse
	if err := c.DoWithContext(ctx, req, &response, reqOpts...); err != nil {
		return nil, err
	}

	// Return the response
	return &response, nil
}

// GetModel retrieves a specific model by name, optionally scoped to a provider.
// If provider is empty, the model is looked up across all providers.
func (c *Client) GetModel(ctx context.Context, name string, opts ...opt.Opt) (*schema.Model, error) {
	if name == "" {
		return nil, fmt.Errorf("model name cannot be empty")
	}

	// Apply options
	o, err := opt.Apply(opts...)
	if err != nil {
		return nil, err
	}

	// Build path: /model/{provider}/{name} or /model/{name}
	req := client.NewRequest()
	var reqOpts []client.RequestOpt
	if provider := o.GetString(opt.ProviderKey); provider != "" {
		reqOpts = append(reqOpts, client.OptPath("model", provider, name))
	} else {
		reqOpts = append(reqOpts, client.OptPath("model", name))
	}

	// Perform request
	var response schema.Model
	if err := c.DoWithContext(ctx, req, &response, reqOpts...); err != nil {
		return nil, err
	}

	// Return the response
	return &response, nil
}
