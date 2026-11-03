/*
elevenlabs implements an API client for ElevenLabs, whose Scribe models
transcribe speech.
https://elevenlabs.io/docs/api-reference/speech-to-text/convert
*/
package elevenlabs

import (
	"context"
	"net/url"

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
	// Endpoint is ElevenLabs' API, which is the default endpoint
	Endpoint = "https://api.elevenlabs.io/v1"

	// apiKeyHeader is the header of the API key, which ElevenLabs has
	// rather than a bearer token
	apiKeyHeader = "xi-api-key"
)

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// New returns a client of ElevenLabs' API, at the endpoint, or ElevenLabs'
// own when it's empty, with the API key, which it needs
func New(endPoint, apiKey string, opts ...client.ClientOpt) (*Client, error) {
	if endPoint == "" {
		endPoint = Endpoint
	}
	if apiKey == "" {
		return nil, llm.ErrBadParameter.With("elevenlabs needs an API key")
	}
	u, err := url.Parse(endPoint)
	if err != nil {
		return nil, err
	} else if u.Scheme == "" || u.Host == "" {
		return nil, llm.ErrBadParameter.Withf("invalid endpoint %q", endPoint)
	}
	client, err := client.New(append(opts, client.OptEndpoint(u.String()), client.OptHeader(apiKeyHeader, apiKey))...)
	if err != nil {
		return nil, err
	}
	return &Client{Client: client, url: u}, nil
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Name returns the provider name
func (*Client) Name() string {
	return llm.ElevenLabs
}

// URL returns the client's endpoint
func (c *Client) URL() *url.URL {
	u := *c.url
	return &u
}

// Self returns the underlying client implementation
func (c *Client) Self() llm.Provider {
	return c
}

// Ping checks the API can be reached, with the API key, by listing its
// models, and returns an error if it can't
func (c *Client) Ping(ctx context.Context) error {
	return c.DoWithContext(ctx, nil, nil, client.OptPath("models"))
}
