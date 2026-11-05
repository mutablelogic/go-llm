// Package httpclient is a client of the heartbeats' HTTP API
package httpclient

import (
	"context"
	"net/http"
	"uuid"

	// Packages
	client "github.com/mutablelogic/go-client"
	schema "github.com/mutablelogic/go-llm/heartbeat/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Client is a client of the heartbeats' HTTP API, which wraps the base HTTP
// client, with typed methods for its requests
type Client struct {
	*client.Client
}

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// New returns a client of the heartbeats' API, at the URL, which is the
// server's API endpoint, such as http://localhost:8084/api, with the options
func New(url string, opts ...client.ClientOpt) (*Client, error) {
	c, err := client.New(append(opts, client.OptEndpoint(url))...)
	if err != nil {
		return nil, err
	}
	return &Client{Client: c}, nil
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// ListHeartbeats returns the heartbeats, the soonest first, as the request
// selects, and how many there are
func (c *Client) ListHeartbeats(ctx context.Context, req schema.HeartbeatListRequest) (*schema.HeartbeatList, error) {
	var response schema.HeartbeatList
	if err := c.DoWithContext(ctx, client.NewRequestEx(http.MethodGet, types.ContentTypeJSON), &response, client.OptPath("heartbeat"), client.OptQuery(req.Query())); err != nil {
		return nil, err
	}
	return &response, nil
}

// CreateHeartbeat creates the heartbeat, and returns it
func (c *Client) CreateHeartbeat(ctx context.Context, meta schema.HeartbeatMeta) (*schema.Heartbeat, error) {
	req, err := client.NewJSONRequestEx(http.MethodPost, meta, types.ContentTypeJSON)
	if err != nil {
		return nil, err
	}
	var response schema.Heartbeat
	if err := c.DoWithContext(ctx, req, &response, client.OptPath("heartbeat")); err != nil {
		return nil, err
	}
	return &response, nil
}

// GetHeartbeat returns the heartbeat with the identifier
func (c *Client) GetHeartbeat(ctx context.Context, id uuid.UUID) (*schema.Heartbeat, error) {
	var response schema.Heartbeat
	if err := c.DoWithContext(ctx, client.NewRequestEx(http.MethodGet, types.ContentTypeJSON), &response, client.OptPath("heartbeat", id.String())); err != nil {
		return nil, err
	}
	return &response, nil
}

// UpdateHeartbeat changes the heartbeat with the identifier, as the request
// says, and returns it
func (c *Client) UpdateHeartbeat(ctx context.Context, id uuid.UUID, change schema.HeartbeatRequest) (*schema.Heartbeat, error) {
	req, err := client.NewJSONRequestEx(http.MethodPatch, change, types.ContentTypeJSON)
	if err != nil {
		return nil, err
	}
	var response schema.Heartbeat
	if err := c.DoWithContext(ctx, req, &response, client.OptPath("heartbeat", id.String())); err != nil {
		return nil, err
	}
	return &response, nil
}

// DeleteHeartbeat deletes the heartbeat with the identifier, and returns it,
// as it was
func (c *Client) DeleteHeartbeat(ctx context.Context, id uuid.UUID) (*schema.Heartbeat, error) {
	var response schema.Heartbeat
	if err := c.DoWithContext(ctx, client.NewRequestEx(http.MethodDelete, types.ContentTypeJSON), &response, client.OptPath("heartbeat", id.String())); err != nil {
		return nil, err
	}
	return &response, nil
}
