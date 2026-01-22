/*
ollama implements an API client for ollama
https://github.com/ollama/ollama/blob/main/docs/api.md
*/
package ollama

import (
	"context"

	// Packages
	client "github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
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
	defaultName = "ollama"
)

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// Create a new client, with an ollama endpoint, which should be something like
// "http://localhost:11434/api"
func New(endPoint string, opts ...client.ClientOpt) (*Client, error) {
	// Create client
	client, err := client.New(append(opts, client.OptEndpoint(endPoint))...)
	if err != nil {
		return nil, err
	}

	// Return the client
	return &Client{client}, nil
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Return the name of the agent
func (*Client) Name() string {
	return defaultName
}

// versionResponse is the response from the version endpoint
type versionResponse struct {
	Version string `json:"version"`
}

// Ping checks if the Ollama server is reachable and returns the version
func (c *Client) Ping(ctx context.Context) (string, error) {
	var response versionResponse
	if err := c.DoWithContext(ctx, nil, &response, client.OptPath("version")); err != nil {
		return "", err
	}
	return response.Version, nil
}
