package httpclient

import (
	"context"
	"net/http"
	"strings"

	// Packages
	client "github.com/mutablelogic/go-client"
	schema "github.com/mutablelogic/go-llm/kernel/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// ListProviders returns a list of providers matching the given request parameters.
func (c *Client) ListProviders(ctx context.Context, req schema.ProviderListRequest) (*schema.ProviderList, error) {
	var response schema.ProviderList
	if err := c.DoWithContext(ctx, client.MethodGet, &response, client.OptPath("provider"), client.OptQuery(req.Query())); err != nil {
		return nil, err
	}

	// Return success
	return &response, nil
}

// CreateProvider creates a new provider with the given insert data.
func (c *Client) CreateProvider(ctx context.Context, req schema.ProviderInsert) (*schema.Provider, error) {
	httpReq, err := client.NewJSONRequest(req)
	if err != nil {
		return nil, err
	}

	var response schema.Provider
	if err := c.DoWithContext(ctx, httpReq, &response, client.OptPath("provider")); err != nil {
		return nil, err
	}

	// Return success
	return &response, nil
}

// GetProvider returns a single provider by name.
func (c *Client) GetProvider(ctx context.Context, name string) (*schema.Provider, error) {
	name = strings.TrimSpace(name)

	var response schema.Provider
	if err := c.DoWithContext(ctx, client.NewRequest(), &response, client.OptPath("provider", name)); err != nil {
		return nil, err
	}

	// Return success
	return &response, nil
}

// DeleteProvider deletes a single provider by name and returns the deleted provider.
func (c *Client) DeleteProvider(ctx context.Context, name string) (*schema.Provider, error) {
	name = strings.TrimSpace(name)

	var response schema.Provider
	if err := c.DoWithContext(ctx, client.MethodDelete, &response, client.OptPath("provider", name)); err != nil {
		return nil, err
	}

	// Return success
	return &response, nil
}

// UpdateProvider patches the writable metadata for a provider by name.
func (c *Client) UpdateProvider(ctx context.Context, name string, req schema.ProviderMeta) (*schema.Provider, error) {
	name = strings.TrimSpace(name)

	httpReq, err := client.NewJSONRequestEx(http.MethodPatch, req, types.ContentTypeAny)
	if err != nil {
		return nil, err
	}

	var response schema.Provider
	if err := c.DoWithContext(ctx, httpReq, &response, client.OptPath("provider", name)); err != nil {
		return nil, err
	}

	// Return success
	return &response, nil
}
