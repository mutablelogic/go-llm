/*
ollama implements an API client for the Ollama provider.
https://github.com/ollama/ollama/tree/main/docs/api
*/
package ollama

import (
	"context"
	"net/url"
	"strings"
	"time"

	// Packages
	client "github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
	modelcache "github.com/mutablelogic/go-llm/pkg/modelcache"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type Client struct {
	*client.Client
	*modelcache.ModelCache
}

var _ llm.Client = (*Client)(nil)
var _ llm.Downloader = (*Client)(nil)

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	defaultEndpoint = "http://localhost:11434/api"
)

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// Create a new client, with an ollama endpoint, which should be something like
// "http://localhost:11434/api".
func New(endPoint string, opts ...client.ClientOpt) (*Client, error) {
	// Default endpoint
	if endPoint == "" {
		endPoint = defaultEndpoint
	}

	// Normalize: if no scheme, treat as host[:port] and add http:// and /api path
	if !strings.Contains(endPoint, "://") {
		endPoint = "http://" + endPoint + "/api"
	}

	// Normalize: if path is empty or just "/", append "/api"
	if u, err := url.Parse(endPoint); err == nil && (u.Path == "" || u.Path == "/") {
		u.Path = "/api"
		endPoint = u.String()
	}

	// Create client
	client, err := client.New(append(opts, client.OptEndpoint(endPoint))...)
	if err != nil {
		return nil, err
	}

	// Return the client
	return &Client{Client: client, ModelCache: modelcache.NewModelCache(time.Minute, 40)}, nil
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Name returns the provider name
func (*Client) Name() string {
	return schema.Ollama
}

// versionResponse is the response from the version endpoint
type versionResponse struct {
	Version string `json:"version"`
}

// Ping checks the connectivity of the client and returns an error if not successful
func (c *Client) Ping(ctx context.Context) error {
	var response versionResponse
	if err := c.DoWithContext(ctx, nil, &response, client.OptPath("version")); err != nil {
		return err
	}
	return nil
}
