// Package httpclient is a client of the memory's HTTP API
package httpclient

import (
	"context"
	"net/http"

	// Packages
	client "github.com/mutablelogic/go-client"
	schema "github.com/mutablelogic/go-llm/memory/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Client is a client of the memory's HTTP API, which wraps the base HTTP
// client, with typed methods for its requests
type Client struct {
	*client.Client
}

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// New returns a client of the memory's API, at the URL, which is the
// server's API endpoint, such as http://localhost:8084/api, with the
// options. Each method adds its own path, such as memory/detail, as the
// memory's handlers register them.
func New(url string, opts ...client.ClientOpt) (*Client, error) {
	c, err := client.New(append(opts, client.OptEndpoint(url))...)
	if err != nil {
		return nil, err
	}
	return &Client{Client: c}, nil
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// GetDetails returns the details, which are those which are known, or
// declined
func (c *Client) GetDetails(ctx context.Context) (*schema.Details, error) {
	var response schema.Details
	if err := c.DoWithContext(ctx, client.NewRequestEx(http.MethodGet, types.ContentTypeJSON), &response, client.OptPath("memory", "detail")); err != nil {
		return nil, err
	}
	return &response, nil
}

// SetDetail sets the detail of the field, which is its value, or that it's
// declined, and returns the details
func (c *Client) SetDetail(ctx context.Context, field schema.Field, detail schema.Detail) (*schema.Details, error) {
	req, err := client.NewJSONRequestEx(http.MethodPut, detail, types.ContentTypeJSON)
	if err != nil {
		return nil, err
	}
	var response schema.Details
	if err := c.DoWithContext(ctx, req, &response, client.OptPath("memory", "detail", string(field))); err != nil {
		return nil, err
	}
	return &response, nil
}

// ForgetDetail forgets the detail of the field, so it isn't known, and
// returns the details
func (c *Client) ForgetDetail(ctx context.Context, field schema.Field) (*schema.Details, error) {
	var response schema.Details
	if err := c.DoWithContext(ctx, client.NewRequestEx(http.MethodDelete, types.ContentTypeJSON), &response, client.OptPath("memory", "detail", string(field))); err != nil {
		return nil, err
	}
	return &response, nil
}
