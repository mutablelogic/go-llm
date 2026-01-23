package mistral

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

var _ llm.Client = (*Client)(nil)

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	endPoint    = "https://api.mistral.ai/v1"
	defaultName = "mistral"
)

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// New creates a new Mistral client using the provided API key.
func New(apiKey string, opts ...client.ClientOpt) (*Client, error) {
	// Create client
	opts = append(opts, client.OptEndpoint(endPoint))
	opts = append(opts, client.OptReqToken(client.Token{Scheme: client.Bearer, Value: apiKey}))

	c, err := client.New(opts...)
	if err != nil {
		return nil, err
	}

	return &Client{Client: c}, nil
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Name returns the provider name.
func (*Client) Name() string {
	return defaultName
}
