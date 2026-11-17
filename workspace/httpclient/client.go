// Package httpclient is a client of the workspaces' API
package httpclient

import (
	// Packages
	client "github.com/mutablelogic/go-client"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Client is a client of the workspaces' API, which wraps the base HTTP
// client, with typed methods
type Client struct {
	*client.Client
}

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// New returns a client of the workspaces' API, at the URL of the shared API
// endpoint, such as "http://localhost:8080/api", with the options, of which
// each method's path is its own "llm/workspace/..." path, as the handlers
// register them
func New(url string, opts ...client.ClientOpt) (*Client, error) {
	c, err := client.New(append(opts, client.OptEndpoint(url))...)
	if err != nil {
		return nil, err
	}
	return &Client{Client: c}, nil
}
