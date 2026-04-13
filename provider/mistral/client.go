/*
mistral implements an API client for the Mistral AI API.
https://docs.mistral.ai/api/
*/
package mistral

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

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	endPoint = "https://api.mistral.ai/v1"
)

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// New creates a new Mistral API client with the given API key
func New(apiKey string, opts ...client.ClientOpt) (*Client, error) {
	opts = append(opts,
		client.OptEndpoint(endPoint),
		client.OptReqToken(client.Token{Scheme: client.Bearer, Value: apiKey}),
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
	return schema.Mistral
}

// Self returns the underlying client implementation.
func (c *Client) Self() llm.Client {
	return c
}

// Ping checks the connectivity of the client and returns an error if not successful
func (*Client) Ping(ctx context.Context) error {
	// TODO: Not implemented for Mistral
	return nil
}
