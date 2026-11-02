// Package httpclient is a client of the registry's HTTP API
package httpclient

import (
	// Packages
	client "github.com/mutablelogic/go-client"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Client is a client of the registry's HTTP API, which wraps the base HTTP
// client, with typed methods for its requests
type Client struct {
	*client.Client
}

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// New returns a client of the registry's API, at the URL, which is the
// server's API endpoint, such as http://localhost:8084/api, with the
// options. Each method adds its own path, such as llm/provider, as the
// registry's handlers register them.
func New(url string, opts ...client.ClientOpt) (*Client, error) {
	c, err := client.New(append(opts, client.OptEndpoint(url))...)
	if err != nil {
		return nil, err
	}
	return &Client{Client: c}, nil
}
