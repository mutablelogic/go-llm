package httpclient

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	// Packages
	client "github.com/mutablelogic/go-client"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// ListConnectors returns registered MCP server connectors, with optional
// filtering by namespace and enabled state, and optional pagination.
func (c *Client) ListConnectors(ctx context.Context, req schema.ListConnectorsRequest) (*schema.ListConnectorsResponse, error) {
	// Build query parameters
	q := make(url.Values)
	if req.Namespace != "" {
		q.Set("namespace", req.Namespace)
	}
	if req.Enabled != nil {
		q.Set("enabled", strconv.FormatBool(*req.Enabled))
	}
	if req.Limit != nil {
		q.Set("limit", strconv.FormatUint(uint64(*req.Limit), 10))
	}
	if req.Offset > 0 {
		q.Set("offset", strconv.FormatUint(uint64(req.Offset), 10))
	}

	// Create request
	reqOpts := []client.RequestOpt{client.OptPath("connector")}
	if len(q) > 0 {
		reqOpts = append(reqOpts, client.OptQuery(q))
	}

	// Perform request
	var response schema.ListConnectorsResponse
	if err := c.DoWithContext(ctx, client.NewRequest(), &response, reqOpts...); err != nil {
		return nil, err
	}

	return &response, nil
}

// GetConnector retrieves the connector registered for the given server URL.
func (c *Client) GetConnector(ctx context.Context, url_ string) (*schema.Connector, error) {
	if url_ == "" {
		return nil, fmt.Errorf("connector URL cannot be empty")
	}

	// Create request
	reqOpts := []client.RequestOpt{client.OptPath("connector", url.PathEscape(url_))}

	// Perform request
	var response schema.Connector
	if err := c.DoWithContext(ctx, client.NewRequest(), &response, reqOpts...); err != nil {
		return nil, err
	}

	return &response, nil
}

// CreateConnector registers a new MCP server connector with the given URL and metadata.
func (c *Client) CreateConnector(ctx context.Context, url_ string, meta schema.ConnectorMeta) (*schema.Connector, error) {
	if url_ == "" {
		return nil, fmt.Errorf("connector URL cannot be empty")
	}

	// Create request
	req, err := client.NewJSONRequest(meta)
	if err != nil {
		return nil, err
	}
	reqOpts := []client.RequestOpt{client.OptPath("connector", url.PathEscape(url_))}

	// Perform request
	var response schema.Connector
	if err := c.DoWithContext(ctx, req, &response, reqOpts...); err != nil {
		return nil, err
	}

	return &response, nil
}

// UpdateConnector updates the metadata for a registered MCP server connector.
func (c *Client) UpdateConnector(ctx context.Context, url_ string, meta schema.ConnectorMeta) (*schema.Connector, error) {
	if url_ == "" {
		return nil, fmt.Errorf("connector URL cannot be empty")
	}

	// Create request
	req, err := client.NewJSONRequestEx(http.MethodPatch, meta, "")
	if err != nil {
		return nil, err
	}
	reqOpts := []client.RequestOpt{client.OptPath("connector", url.PathEscape(url_))}

	// Perform request
	var response schema.Connector
	if err := c.DoWithContext(ctx, req, &response, reqOpts...); err != nil {
		return nil, err
	}

	return &response, nil
}

// DeleteConnector removes the connector registered for the given server URL.
func (c *Client) DeleteConnector(ctx context.Context, url_ string) error {
	if url_ == "" {
		return fmt.Errorf("connector URL cannot be empty")
	}

	// Create request
	reqOpts := []client.RequestOpt{client.OptPath("connector", url.PathEscape(url_))}

	// Perform request
	if err := c.DoWithContext(ctx, client.MethodDelete, nil, reqOpts...); err != nil {
		return err
	}

	return nil
}
