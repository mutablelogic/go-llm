/*
mistral implements an API client for mistral (https://docs.mistral.ai/api/)
*/
package mistral

import (
	// Packages
	"context"

	"github.com/mutablelogic/go-client"
	"github.com/mutablelogic/go-llm"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type Client struct {
	*client.Client
}

var _ llm.Agent = (*Client)(nil)

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	endPoint    = "https://api.mistral.ai/v1"
	defaultName = "mistral"
)

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// Create a new client
func New(ApiKey string, opts ...client.ClientOpt) (*Client, error) {
	// Create client
	opts = append(opts, client.OptEndpoint(endPoint))
	opts = append(opts, client.OptReqToken(client.Token{
		Scheme: client.Bearer,
		Value:  ApiKey,
	}))
	client, err := client.New(opts...)
	if err != nil {
		return nil, err
	}

	// Return the client
	return &Client{client}, nil
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Return the name of the agent
func (Client) Name() string {
	return defaultName
}

// Return the models
func (c *Client) Models(ctx context.Context) ([]llm.Model, error) {
	return c.ListModels(ctx)
}

// Return a model by name, or nil if not found.
// Panics on error.
func (c *Client) Model(ctx context.Context, name string) llm.Model {
	return nil
}
