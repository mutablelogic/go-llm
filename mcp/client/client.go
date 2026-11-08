package client

import (
	"net/url"
	"strings"

	// Packages
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	goclient "github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
	version "github.com/mutablelogic/go-server/pkg/version"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Client is a client of a remote MCP server, at its URL, whose requests are
// made with go-client
type Client struct {
	*goclient.Client

	// The URL of the server
	endpoint string

	// The name, and version, of the client, which the server is told, as it
	// connects
	implementation sdkmcp.Implementation

	// The options of go-client, which are gathered as the options are
	// applied, and used once, as the client is created
	opts []goclient.ClientOpt
}

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

// DefaultName is the name of the client, which the server is told, unless
// it's given another, with WithName
const DefaultName = "go-llm"

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// New returns a client of the MCP server at the URL, such as
// https://mcp.example.com/mcp, or one which ends /sse, of a server with the
// SSE transport, with the options, such as go-client's, with
// WithClientOpts. It isn't connected. Returns ErrBadParameter when the URL
// isn't an http, or https, URL with a host.
func New(endpoint string, opts ...Opt) (*Client, error) {
	u, err := url.Parse(strings.TrimSpace(endpoint))
	if err != nil {
		return nil, llm.ErrBadParameter.Withf("url %q: %v", endpoint, err)
	} else if u.Scheme != "http" && u.Scheme != "https" {
		return nil, llm.ErrBadParameter.Withf("url %q isn't http, or https", endpoint)
	} else if u.Host == "" {
		return nil, llm.ErrBadParameter.Withf("url %q has no host", endpoint)
	}

	// Apply the options, which gather go-client's
	c := &Client{endpoint: u.String(), implementation: sdkmcp.Implementation{Name: DefaultName, Version: version.Version()}}
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		if err := opt(c); err != nil {
			return nil, err
		}
	}

	// Create go-client's client, for the server, with its options
	client, err := goclient.New(append([]goclient.ClientOpt{goclient.OptEndpoint(c.endpoint)}, c.opts...)...)
	if err != nil {
		return nil, err
	}
	c.Client, c.opts = client, nil
	return c, nil
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Endpoint returns the URL of the server
func (c *Client) Endpoint() string {
	return c.endpoint
}
