package httpclient

import (
	"context"
	"net/http"
	"sync"

	// Packages
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	client "github.com/mutablelogic/go-client"
	transport "github.com/mutablelogic/go-client/pkg/transport"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Client is an MCP HTTP client that wraps the base HTTP client
// and provides typed methods for interacting with the MCP server.
type Client struct {
	*client.Client
	sdkmcp.Implementation

	// MCP session state
	url    string
	authFn func(context.Context, string) error

	session *sdkmcp.ClientSession
	mu      sync.Mutex
}

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// New creates a new HTTP client with the given base URL and options.
// The url parameter should point to the MCP server endpoint, e.g.
// https://mcp.asana.com/sse
//
// authFn is called when the server returns 401 to perform the OAuth flow.
// Pass nil to disable auth.
func New(url, name, version string, authFn func(context.Context, string) error, opts ...client.ClientOpt) (*Client, error) {
	c := new(Client)

	// Install a token transport via OptTransport so it is wired into the
	// transport chain during client.New. The closure captures c (a pointer)
	// so AccessToken() is resolved lazily at request time, after c.Client
	// has been assigned below.
	tokenOpt := client.OptTransport(func(parent http.RoundTripper) http.RoundTripper {
		return transport.NewToken(parent, func() string {
			if c.Client == nil {
				return ""
			}
			return c.Client.AccessToken()
		})
	})

	// Create the client; this does not establish the session yet. Call Run() to connect and drive the session.
	if cl, err := client.New(append(opts, client.OptEndpoint(url), tokenOpt)...); err != nil {
		return nil, err
	} else {
		c.Client = cl
		c.Implementation = sdkmcp.Implementation{Name: name, Version: version}
		c.url = url
		c.authFn = authFn
	}

	// Return the client; caller should call Run to connect and drive the session.
	return c, nil
}
