package client

import (
	"context"
	"net/url"
	"strings"
	"sync"

	// Packages
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	client "github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Client is a client of a remote MCP server, at its URL, whose requests are
// made with go-client
type Client struct {
	*client.Client

	// The URL of the server
	endpoint string

	// The name, and version, of the client, which the server is told, as it
	// connects, with WithName
	implementation sdkmcp.Implementation

	// Called when it's connected, or disconnected, or nil
	change ChangeFunc

	// The session with the server, once it's connected, and the error of
	// the last connection, or of the session, when it ended, which are held
	// while it's connected, and disconnected
	mu      sync.Mutex
	session *sdkmcp.ClientSession
	err     error

	// Cancels the context of the session, which its transport, such as
	// SSE's stream, lasts as long as, once it's disconnected, or it ends
	cancel context.CancelFunc
}

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// New returns a client of the MCP server at the URL, such as
// https://mcp.example.com/mcp, or one which ends /sse, of a server with the
// SSE transport, with the options, such as its name, and version, which the
// server is told, with WithName, and go-client's, with WithClientOpts. It
// isn't connected, until Connect is called. Returns ErrBadParameter when
// the URL isn't an http, or https, URL with a host.
func New(endpoint string, opts ...Opt) (*Client, error) {
	u, err := url.Parse(strings.TrimSpace(endpoint))
	if err != nil {
		return nil, llm.ErrBadParameter.Withf("url %q: %v", endpoint, err)
	} else if u.Scheme != "http" && u.Scheme != "https" {
		return nil, llm.ErrBadParameter.Withf("url %q isn't http, or https", endpoint)
	} else if u.Host == "" {
		return nil, llm.ErrBadParameter.Withf("url %q has no host", endpoint)
	}

	// Apply the options
	var o opt
	if err := o.apply(opts); err != nil {
		return nil, err
	}

	// Create go-client's client, for the server, with its options
	httpClient, err := client.New(append([]client.ClientOpt{client.OptEndpoint(u.String())}, o.clientopts...)...)
	if err != nil {
		return nil, err
	}
	return &Client{
		Client:         httpClient,
		endpoint:       u.String(),
		implementation: sdkmcp.Implementation{Name: o.name, Version: o.version},
		change:         o.change,
	}, nil
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Endpoint returns the URL of the server
func (c *Client) Endpoint() string {
	return c.endpoint
}
