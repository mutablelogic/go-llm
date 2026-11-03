/*
ollama implements an API client for the Ollama provider.
https://github.com/ollama/ollama/tree/main/docs/api
*/
package ollama

import (
	"context"
	"net"
	"net/url"
	"strings"
	"sync"

	// Packages
	client "github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type Client struct {
	*client.Client
	url *url.URL

	// The models which have been shown, by their names, with their digests,
	// so a model which hasn't changed isn't shown again, when the models are
	// listed
	mu    sync.Mutex
	shown map[string]apiModel
}

var _ llm.Provider = (*Client)(nil)

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	defaultEndpoint = "http://localhost:11434/api"
	defaultPort     = "11434"
)

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// Create a new client, with an ollama endpoint, which should be something like
// "http://localhost:11434/api". As with OLLAMA_HOST, an endpoint without a
// scheme, such as "localhost", uses http and Ollama's default port, 11434,
// unless it has a port.
func New(endPoint string, opts ...client.ClientOpt) (*Client, error) {
	// Default endpoint
	if endPoint == "" {
		endPoint = defaultEndpoint
	}

	// Normalize: if no scheme, treat as host[:port], with http:// and the
	// default port
	scheme := strings.Contains(endPoint, "://")
	if !scheme {
		endPoint = "http://" + endPoint
	}
	u, err := url.Parse(endPoint)
	if err != nil {
		return nil, err
	}
	if !scheme && u.Port() == "" && u.Hostname() != "" {
		u.Host = net.JoinHostPort(u.Hostname(), defaultPort)
	}

	// Normalize: if path is empty or just "/", append "/api"
	if u.Path == "" || u.Path == "/" {
		u.Path = "/api"
	}

	// Create client
	client, err := client.New(append(opts, client.OptEndpoint(u.String()))...)
	if err != nil {
		return nil, err
	}

	// Return the client
	return &Client{Client: client, url: u}, nil
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Name returns the provider name
func (*Client) Name() string {
	return llm.Ollama
}

// URL returns the client's normalized endpoint
func (c *Client) URL() *url.URL {
	u := *c.url
	return &u
}

// Self returns the underlying client implementation.
func (c *Client) Self() llm.Provider {
	return c
}

// Ping checks the connectivity of the client and returns an error if not successful
func (c *Client) Ping(ctx context.Context) error {
	_, error := c.Version(ctx)
	return error
}
