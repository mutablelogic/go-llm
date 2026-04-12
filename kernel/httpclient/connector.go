package httpclient

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	// Packages
	client "github.com/mutablelogic/go-client"
	schema "github.com/mutablelogic/go-llm/kernel/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// ListConnectors returns connectors matching the given request parameters.
func (c *Client) ListConnectors(ctx context.Context, req schema.ConnectorListRequest) (*schema.ConnectorList, error) {
	var response schema.ConnectorList
	if err := c.DoWithContext(ctx, client.MethodGet, &response, client.OptPath("connector"), client.OptQuery(req.Query())); err != nil {
		return nil, err
	}

	return &response, nil
}

// GetConnector returns a connector by URL.
func (c *Client) GetConnector(ctx context.Context, rawURL string) (*schema.Connector, error) {
	if rawURL == "" {
		return nil, fmt.Errorf("connector URL cannot be empty")
	}

	var response schema.Connector
	if err := c.DoWithContext(ctx, client.MethodGet, &response, client.OptPath("connector", url.PathEscape(rawURL))); err != nil {
		return nil, err
	}

	return &response, nil
}

// CreateConnector creates a new connector with the given insert data.
func (c *Client) CreateConnector(ctx context.Context, req schema.ConnectorInsert) (*schema.Connector, error) {
	if req.URL == "" {
		return nil, fmt.Errorf("connector URL cannot be empty")
	}

	httpReq, err := client.NewJSONRequest(req)
	if err != nil {
		return nil, err
	}

	var response schema.Connector
	if err := c.DoWithContext(ctx, httpReq, &response, client.OptPath("connector")); err != nil {
		return nil, err
	}

	return &response, nil
}

// UpdateConnector patches connector metadata for the given URL.
func (c *Client) UpdateConnector(ctx context.Context, rawURL string, req schema.ConnectorMeta) (*schema.Connector, error) {
	if rawURL == "" {
		return nil, fmt.Errorf("connector URL cannot be empty")
	}

	httpReq, err := client.NewJSONRequestEx(http.MethodPatch, req, types.ContentTypeAny)
	if err != nil {
		return nil, err
	}

	var response schema.Connector
	if err := c.DoWithContext(ctx, httpReq, &response, client.OptPath("connector", url.PathEscape(rawURL))); err != nil {
		return nil, err
	}

	return &response, nil
}

// DeleteConnector deletes a connector and returns the deleted connector.
func (c *Client) DeleteConnector(ctx context.Context, rawURL string) (*schema.Connector, error) {
	if rawURL == "" {
		return nil, fmt.Errorf("connector URL cannot be empty")
	}

	var response schema.Connector
	if err := c.DoWithContext(ctx, client.MethodDelete, &response, client.OptPath("connector", url.PathEscape(rawURL))); err != nil {
		return nil, err
	}

	return &response, nil
}
