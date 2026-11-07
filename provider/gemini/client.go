/*
gemini implements an API client for Google's Gemini API, which lists its
models, and counts tokens.
https://ai.google.dev/api
*/
package gemini

import (
	"context"
	"net/url"
	"strings"
	"sync"
	"time"

	// Packages
	client "github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Client is a client of the Gemini API, which lists its models, and counts
// the tokens of text
type Client struct {
	*client.Client
	url *url.URL

	// When the API was last pinged, and the error from the ping, which is
	// held while the API is pinged
	mu       sync.Mutex
	lastPing time.Time
	pingErr  error

	// The tokens of text which were counted, by model, and the tokens of
	// what's counted with text, by model
	counts tokenCounts
}

var _ llm.Provider = (*Client)(nil)

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// Endpoint is the endpoint of the Gemini API, which is the default
	Endpoint = "https://generativelanguage.googleapis.com/v1beta"

	// PingInterval is how long the result of pinging the API is kept, before
	// it is pinged again, so the API isn't pinged too often
	PingInterval = 5 * time.Minute
)

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// New returns a client of the Gemini API, at the endpoint, which is
// Endpoint when it's empty, with the API key, which it needs, and which is
// sent in a header, rather than the URL, so it isn't logged
func New(endPoint, apiKey string, opts ...client.ClientOpt) (*Client, error) {
	if apiKey == "" {
		return nil, llm.ErrBadParameter.With("gemini needs an API key")
	}

	// Normalize the endpoint: a host, without a scheme, is https, and a URL
	// without a path is at /v1beta
	if endPoint == "" {
		endPoint = Endpoint
	} else if !strings.Contains(endPoint, "://") {
		endPoint = "https://" + endPoint
	}
	u, err := url.Parse(endPoint)
	if err != nil {
		return nil, err
	} else if u.Path == "" || u.Path == "/" {
		u.Path = "/v1beta"
	}

	// Send the API key with each request
	opts = append(opts,
		client.OptEndpoint(u.String()),
		client.OptHeader("x-goog-api-key", apiKey),
	)
	c, err := client.New(opts...)
	if err != nil {
		return nil, err
	}
	return &Client{Client: c, url: u}, nil
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Name returns the provider name
func (*Client) Name() string {
	return llm.Gemini
}

// URL returns the client's normalized endpoint
func (c *Client) URL() *url.URL {
	u := *c.url
	return &u
}

// Self returns the underlying client implementation
func (c *Client) Self() llm.Provider {
	return c
}

// Ping checks the connectivity of the client, and that the API key is
// accepted, by listing a model. The API is only pinged once every ping
// interval: otherwise, the result of the last ping is returned, unless it
// ended as its caller's context did.
func (c *Client) Ping(ctx context.Context) error {
	// Concurrent pings wait for the one in progress
	c.mu.Lock()
	defer c.mu.Unlock()

	// Return the last result, if it's recent
	if !c.lastPing.IsZero() && time.Since(c.lastPing) < PingInterval {
		return c.pingErr
	}

	// Ping the API, and keep the result, unless the ping ended as its
	// caller gave up, which says nothing of the API, so the next ping asks
	// it again
	start := time.Now()
	_, err := c.listModelsPage(ctx, "", 1)
	if ctx.Err() != nil {
		return err
	}
	c.pingErr, c.lastPing = err, start
	return err
}
