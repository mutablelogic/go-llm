package httpclient

import (
	"context"
	"net/http"

	// Packages
	client "github.com/mutablelogic/go-client"
	schema "github.com/mutablelogic/go-llm/registry/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// ListProviders returns the providers, in order of their names, of the type
// which the request selects, or every type, with whether each can be
// reached
func (c *Client) ListProviders(ctx context.Context, req schema.ProviderListRequest) (*schema.ProviderList, error) {
	var response schema.ProviderList
	if err := c.DoWithContext(ctx, client.MethodGet, &response, client.OptPath("llm", "provider"), client.OptQuery(req.Query())); err != nil {
		return nil, err
	}
	return &response, nil
}

// GetProvider returns the provider with the name, with whether it can be
// reached
func (c *Client) GetProvider(ctx context.Context, name string) (*schema.ProviderStatus, error) {
	var response schema.ProviderStatus
	if err := c.DoWithContext(ctx, client.MethodGet, &response, client.OptPath("llm", "provider", name)); err != nil {
		return nil, err
	}
	return &response, nil
}

// CreateProvider creates a provider of the type, with its name, endpoint
// and API key, which can be a credential's placeholder, such as
// ${OPENAI_KEY}, and returns it, once it's reached
func (c *Client) CreateProvider(ctx context.Context, provider schema.Provider) (*schema.ProviderStatus, error) {
	req, err := client.NewJSONRequest(provider)
	if err != nil {
		return nil, err
	}
	var response schema.ProviderStatus
	if err := c.DoWithContext(ctx, req, &response, client.OptPath("llm", "provider")); err != nil {
		return nil, err
	}
	return &response, nil
}

// UpdateProvider replaces the fields of the provider with the name with the
// meta, keeping its API key when the meta has none, and returns it
func (c *Client) UpdateProvider(ctx context.Context, name string, meta schema.ProviderMeta) (*schema.ProviderStatus, error) {
	req, err := client.NewJSONRequestEx(http.MethodPatch, meta, types.ContentTypeAny)
	if err != nil {
		return nil, err
	}
	var response schema.ProviderStatus
	if err := c.DoWithContext(ctx, req, &response, client.OptPath("llm", "provider", name)); err != nil {
		return nil, err
	}
	return &response, nil
}

// DeleteProvider deletes the provider with the name from the registry, and
// returns it
func (c *Client) DeleteProvider(ctx context.Context, name string) (*schema.ProviderStatus, error) {
	var response schema.ProviderStatus
	if err := c.DoWithContext(ctx, client.MethodDelete, &response, client.OptPath("llm", "provider", name)); err != nil {
		return nil, err
	}
	return &response, nil
}
