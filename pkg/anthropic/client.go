/*
anthropic implements an API client for anthropic (https://docs.anthropic.com/en/api/getting-started)
*/
package anthropic

import (
	// Packages
	"context"

	client "github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
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
	endPoint       = "https://api.anthropic.com/v1"
	defaultVersion = "2023-06-01"
	defaultName    = "anthropic"
)

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// Create a new client
func New(ApiKey string, opts ...client.ClientOpt) (*Client, error) {
	// Create client
	opts = append(opts, client.OptEndpoint(endPoint))
	opts = append(opts, client.OptHeader("x-api-key", ApiKey), client.OptHeader("anthropic-version", defaultVersion))
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
func (*Client) Name() string {
	return defaultName
}

// Return the models
func (anthropic *Client) Models(ctx context.Context) ([]llm.Model, error) {
	// Cache models
	if anthropic.cache == nil {
		models, err := anthropic.ListModels(ctx)
		if err != nil {
			return nil, err
		}
		anthropic.cache = make(map[string]llm.Model, len(models))
		for _, model := range models {
			anthropic.cache[model.Name()] = model
		}
	}

	// Return models
	result := make([]llm.Model, 0, len(anthropic.cache))
	for _, model := range anthropic.cache {
		result = append(result, model)
	}
	return result, nil
}

// Return a model by name, or nil if not found.
// Panics on error.
func (anthropic *Client) Model(ctx context.Context, name string) llm.Model {
	if anthropic.cache == nil {
		if _, err := anthropic.Models(ctx); err != nil {
			panic(err)
		}
	}
	return anthropic.cache[name]
}
