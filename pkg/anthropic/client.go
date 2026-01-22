/*
anthropic implements an API client for anthropic
https://docs.anthropic.com/en/api/getting-started
*/
package anthropic

import (
	// Packages
	client "github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type Client struct {
	*client.Client
	apiKey string
}

var _ llm.Client = (*Client)(nil)

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
	opts = append(opts, client.OptHeader("x-api-key", ApiKey))
	opts = append(opts, client.OptHeader("anthropic-version", defaultVersion))
	client, err := client.New(opts...)
	if err != nil {
		return nil, err
	}

	// Return the client
	return &Client{client, ApiKey}, nil
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Return the name of the agent
func (*Client) Name() string {
	return defaultName
}
