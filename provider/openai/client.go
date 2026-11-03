/*
openai implements an API client for OpenAI, and for other providers with an
OpenAI-compatible API, such as vLLM, llama.cpp and LM Studio.
https://platform.openai.com/docs/api-reference
*/
package openai

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

type Client struct {
	*client.Client
	url *url.URL

	// When the API was last pinged, and the error from the ping, which is
	// held while the API is pinged
	mu       sync.Mutex
	lastPing time.Time
	pingErr  error
}

var _ llm.Provider = (*Client)(nil)

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// OpenAIEndpoint is the endpoint of OpenAI's API, which is the default
	OpenAIEndpoint = "https://api.openai.com/v1"

	// DeepSeekEndpoint is the endpoint of DeepSeek's OpenAI-compatible API
	DeepSeekEndpoint = "https://api.deepseek.com"

	// MistralEndpoint is the endpoint of Mistral's OpenAI-compatible API
	MistralEndpoint = "https://api.mistral.ai/v1"

	// PingInterval is how long the result of pinging the API is kept, before
	// it is pinged again, so that third-party APIs aren't pinged too often
	PingInterval = 5 * time.Minute
)

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// Create a new client, with an OpenAI-compatible endpoint, such as
// "https://api.openai.com/v1" (the default), and an API key, which is sent as
// a bearer token. The key may be empty, for servers which don't need one.
func New(endPoint, apiKey string, opts ...client.ClientOpt) (*Client, error) {
	// Default endpoint
	if endPoint == "" {
		endPoint = OpenAIEndpoint
	}

	// Normalize: if no scheme, treat as host[:port] and add https:// and /v1 path
	if !strings.Contains(endPoint, "://") {
		endPoint = "https://" + endPoint + "/v1"
	}

	// Normalize: if path is empty or just "/", append "/v1"
	if u, err := url.Parse(endPoint); err == nil && (u.Path == "" || u.Path == "/") {
		u.Path = "/v1"
		endPoint = u.String()
	}

	// Send the API key with each request
	if apiKey != "" {
		opts = append(opts, client.OptReqToken(client.Token{Scheme: client.Bearer, Value: apiKey}))
	}

	// Parse the normalized endpoint
	u, err := url.Parse(endPoint)
	if err != nil {
		return nil, err
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
	return llm.OpenAI
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

// Ping checks the connectivity of the client, and that the API key is
// accepted, by listing the models. OpenAI-compatible APIs have no separate
// health or version endpoint. The API is only pinged once every ping
// interval: otherwise, the result of the last ping is returned.
func (c *Client) Ping(ctx context.Context) error {
	// Concurrent pings wait for the one in progress
	c.mu.Lock()
	defer c.mu.Unlock()

	// Return the last result, if it's recent
	if !c.lastPing.IsZero() && time.Since(c.lastPing) < PingInterval {
		return c.pingErr
	}

	// Ping the API, and keep the result
	start := time.Now()
	_, c.pingErr = c.listModels(ctx)
	c.lastPing = start
	return c.pingErr
}
