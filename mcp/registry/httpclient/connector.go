package httpclient

import (
	"context"
	"net/http"

	// Packages
	client "github.com/mutablelogic/go-client"
	schema "github.com/mutablelogic/go-llm/mcp/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// ListConnectors returns the connectors, with their status, in order of
// their names
func (c *Client) ListConnectors(ctx context.Context) (*schema.ConnectorList, error) {
	var response schema.ConnectorList
	if err := c.DoWithContext(ctx, client.NewRequestEx(http.MethodGet, types.ContentTypeJSON), &response, client.OptPath("mcp", "connector")); err != nil {
		return nil, err
	}
	return &response, nil
}

// GetConnector returns the connector with the name, with its status
func (c *Client) GetConnector(ctx context.Context, name string) (*schema.Connector, error) {
	var response schema.Connector
	if err := c.DoWithContext(ctx, client.NewRequestEx(http.MethodGet, types.ContentTypeJSON), &response, client.OptPath("mcp", "connector", name)); err != nil {
		return nil, err
	}
	return &response, nil
}

// CreateConnector creates the connector, with its name, and URL, and
// returns it, with its serial. Its server is connected by the server.
func (c *Client) CreateConnector(ctx context.Context, connector schema.Connector) (*schema.Connector, error) {
	req, err := client.NewJSONRequestEx(http.MethodPost, connector.Definition(), types.ContentTypeJSON)
	if err != nil {
		return nil, err
	}
	var response schema.Connector
	if err := c.DoWithContext(ctx, req, &response, client.OptPath("mcp", "connector")); err != nil {
		return nil, err
	}
	return &response, nil
}

// UpdateConnector replaces the fields of the connector with the name, such
// as its URL, and returns it, with its new serial
func (c *Client) UpdateConnector(ctx context.Context, name string, meta schema.ConnectorMeta) (*schema.Connector, error) {
	req, err := client.NewJSONRequestEx(http.MethodPatch, meta, types.ContentTypeJSON)
	if err != nil {
		return nil, err
	}
	var response schema.Connector
	if err := c.DoWithContext(ctx, req, &response, client.OptPath("mcp", "connector", name)); err != nil {
		return nil, err
	}
	return &response, nil
}

// DeleteConnector deletes the connector with the name, and returns it, as
// it was
func (c *Client) DeleteConnector(ctx context.Context, name string) (*schema.Connector, error) {
	var response schema.Connector
	if err := c.DoWithContext(ctx, client.NewRequestEx(http.MethodDelete, types.ContentTypeJSON), &response, client.OptPath("mcp", "connector", name)); err != nil {
		return nil, err
	}
	return &response, nil
}
