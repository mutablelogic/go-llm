/*
mistral implements an API client for mistral
https://docs.mistral.ai/api/
*/
package mistral

import (
	// Packages
	client "github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
	impl "github.com/mutablelogic/go-llm/pkg/internal/impl"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type Client struct {
	*client.Client
	*impl.ModelCache
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
	return &Client{client, impl.NewModelCache()}, nil
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Return the name of the agent
func (Client) Name() string {
	return defaultName
}
