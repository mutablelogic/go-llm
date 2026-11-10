/*
elevenlabs implements an API client for ElevenLabs, whose Scribe models
transcribe speech, and whose other models speak text, in the account's
voices.
https://elevenlabs.io/docs/api-reference
*/
package elevenlabs

import (
	"context"
	"net/url"
	"sync"
	"time"

	// Packages
	client "github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type Client struct {
	*client.Client
	url *url.URL

	// The last ping, and its result, which are kept for PingInterval
	mu       sync.Mutex
	lastPing time.Time
	pingErr  error

	// The models which speak, and when they were listed, which are kept for
	// PingInterval
	speechMu sync.Mutex
	speech   []apiModel
	listed   time.Time

	// The account's voices, and when they were listed, which are kept for
	// voicesTTL, to find a voice by its title
	voicesMu sync.Mutex
	cached   []llm.Voice
	voicesAt time.Time
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

// PingInterval is how long the result of pinging the API is kept, before it
// is pinged again, so that a third-party API isn't pinged too often
var PingInterval = 5 * time.Minute

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
// models, and returns an error if it can't. The API is only pinged once
// every PingInterval: otherwise, the result of the last ping is returned,
// unless it ended as its caller's context did.
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
	err := c.DoWithContext(ctx, nil, nil, client.OptPath("models"))
	if ctx.Err() != nil {
		return err
	}
	c.pingErr, c.lastPing = err, start
	return err
}
