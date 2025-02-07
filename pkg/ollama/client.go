/*
ollama implements an API client for ollama
https://github.com/ollama/ollama/blob/main/docs/api.md
*/
package ollama

import (
	// Packages
	client "github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
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
