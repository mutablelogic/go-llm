package httpclient

import (
	// Packages
	authclient "github.com/djthorpe/go-auth/pkg/httpclient/auth"
	client "github.com/mutablelogic/go-client"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Client is an LLM HTTP client that wraps the base HTTP client
// and provides typed methods for interacting with the LLM API.
type Client struct {
	*authclient.Client
}

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// New creates a new LLM HTTP client with the given base URL and options.
// The url parameter should point to the LLM API endpoint, e.g.
// "http://localhost:8084/api".
func New(url string, opts ...client.ClientOpt) (*Client, error) {
	c := new(Client)
	if client, err := authclient.New(url, opts...); err != nil {
		return nil, err
	} else {
		c.Client = client
	}
	return c, nil
}
