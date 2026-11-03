package httpclient

import (
	// Packages
	client "github.com/mutablelogic/go-client"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Client is an llm toolkit HTTP client that wraps the base HTTP client
// and provides typed methods for interacting with the llm toolkit API.
type Client struct {
	*client.Client
	endpoint string
}

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// New creates a new llm toolkit HTTP client with the given base URL and
// options. The url parameter should point to the shared API endpoint (the
// same one the registry, filer and credential clients use), e.g.
// "http://localhost:8080/api" - each method includes its own "llm/..." path
// segment, matching how httphandler registers its routes.
func New(url string, opts ...client.ClientOpt) (*Client, error) {
	c := &Client{endpoint: url}
	cl, err := client.New(append(opts, client.OptEndpoint(url))...)
	if err != nil {
		return nil, err
	}
	c.Client = cl
	return c, nil
}
