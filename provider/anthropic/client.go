/*
anthropic implements an API client for the Anthropic Messages API.
https://docs.anthropic.com/en/api/getting-started
*/
package anthropic

import (
	"context"

	// Packages
	client "github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/kernel/schema"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type Client struct {
	*client.Client
}

var _ llm.Client = (*Client)(nil)
var _ llm.Generator = (*Client)(nil)

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	endPoint   = "https://api.anthropic.com/v1"
	apiVersion = "2023-06-01"
)

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// New creates a new Anthropic API client with the given API key
func New(apiKey string, opts ...client.ClientOpt) (*Client, error) {
	opts = append(opts,
		client.OptEndpoint(endPoint),
		client.OptHeader("x-api-key", apiKey),
		client.OptHeader("anthropic-version", apiVersion),
	)
	if c, err := client.New(opts...); err != nil {
		return nil, err
	} else {
		return &Client{c}, nil
	}
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Name returns the provider name
func (*Client) Name() string {
	return schema.Anthropic
}

// Self returns the underlying client implementation.
func (c *Client) Self() llm.Client {
	return c
}

// Ping checks the connectivity of the client and returns an error if not successful
func (c *Client) Ping(ctx context.Context) error {
	// TODO: Not implemented for Anthropic
	return nil
}
