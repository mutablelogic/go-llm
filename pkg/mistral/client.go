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
	cache map[string]llm.Model
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
	return &Client{client, nil}, nil
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Return the name of the agent
func (Client) Name() string {
	return defaultName
}

// Return the models
func (c *Client) Models(ctx context.Context) ([]llm.Model, error) {
	// Cache models
	if c.cache == nil {
		models, err := c.ListModels(ctx)
		if err != nil {
			return nil, err
		}
		c.cache = make(map[string]llm.Model, len(models))
		for _, model := range models {
			c.cache[model.Name()] = model
		}
	}

	// Return models
	result := make([]llm.Model, 0, len(c.cache))
	for _, model := range c.cache {
		result = append(result, model)
	}
	return result, nil
}

// Return a model by name, or nil if not found.
// Panics on error.
func (c *Client) Model(ctx context.Context, name string) llm.Model {
	if c.cache == nil {
		if _, err := c.Models(ctx); err != nil {
			panic(err)
		}
	}
	return c.cache[name]
}
