/*
gowhisper implements an API client for go-whisper, which transcribes, and
translates, speech with whisper.cpp models, which it downloads and stores.
https://github.com/mutablelogic/go-whisper
*/
package gowhisper

import (
	"context"
	"net"
	"net/url"
	"strings"

	// Packages
	client "github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type Client struct {
	*client.Client
	url *url.URL
}

var _ llm.Provider = (*Client)(nil)

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	defaultEndpoint = "http://localhost:8081/api/whisper"
	defaultPort     = "8081"
	defaultPath     = "/api/whisper"
)

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// New returns a client of a go-whisper server, with its endpoint, which
// should be something like "http://localhost:8081/api/whisper". An endpoint
// without a scheme, such as "localhost", uses http and go-whisper's default
// port, 8081, unless it has a port, and one without a path uses
// go-whisper's, "/api/whisper". go-whisper has no API key.
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

	// Normalize: if path is empty or just "/", use go-whisper's
	if u.Path == "" || u.Path == "/" {
		u.Path = defaultPath
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
	return llm.GoWhisper
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

// Ping checks the server can be reached, by listing its models, as
// go-whisper has no version, and returns an error if it can't
func (c *Client) Ping(ctx context.Context) error {
	_, err := c.listModels(ctx)
	return err
}
