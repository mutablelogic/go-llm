/*
google implements an API client for the Google Gemini REST API.
https://ai.google.dev/gemini-api/docs
*/
package google

import (
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
var _ llm.Generator = (*Client)(nil)

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	endPoint    = "https://generativelanguage.googleapis.com/v1beta"
	defaultName = schema.Gemini
)

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// New creates a new Google Gemini API client with the given API key
func New(apiKey string, opts ...client.ClientOpt) (*Client, error) {
	opts = append(opts,
		client.OptEndpoint(endPoint),
		client.OptHeader("x-goog-api-key", apiKey),
	)
	if c, err := client.New(opts...); err != nil {
		return nil, err
	} else {
		return &Client{c, modelcache.NewModelCache(time.Hour, 40)}, nil
	}
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Name returns the provider name
func (*Client) Name() string {
	return defaultName
}
