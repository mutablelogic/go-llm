package httpclient

import (
	"context"
	"sync"

	// Packages
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	client "github.com/mutablelogic/go-client"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Client is an MCP HTTP client that wraps the base HTTP client
// and provides typed methods for interacting with the MCP server.
type Client struct {
	*client.Client
	sdkmcp.Implementation

	// MCP session state
	url       string
	session   *sdkmcp.ClientSession
	runCancel context.CancelFunc
	runWg     sync.WaitGroup
	mu        sync.Mutex
}

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// New creates a new HTTP client with the given base URL and options.
// The url parameter should point to the MCP server endpoint, e.g.
// "http://localhost:8084/api".
func New(url, name, version string, opts ...client.ClientOpt) (*Client, error) {
	c := new(Client)
	if cl, err := client.New(append(opts, client.OptEndpoint(url))...); err != nil {
		return nil, err
	} else {
		c.Client = cl
		c.Implementation = sdkmcp.Implementation{Name: name, Version: version}
		c.url = url
	}

	// Return the client without connecting; caller should call Connect to start the session.
	return c, nil
}
