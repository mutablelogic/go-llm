/*
gemini implements an API client for Google Gemini
https://ai.google.dev/gemini-api/docs
*/
package gemini

import (
	// Packages
	"time"

	client "github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
	"github.com/mutablelogic/go-llm/pkg/agent"
	opt "github.com/mutablelogic/go-llm/pkg/opt"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Client is a client for the Google Gemini API
type Client struct {
	*client.Client
	*agent.ModelCache
}

var _ llm.Client = (*Client)(nil)
var _ llm.ToolOptioner = (*Client)(nil)

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	endPoint    = "https://generativelanguage.googleapis.com/v1beta"
	defaultName = "google"
)

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// New creates a new Google Gemini client with the given API key
func New(apiKey string, opts ...client.ClientOpt) (*Client, error) {
	// Create client
	opts = append(opts, client.OptEndpoint(endPoint))
	opts = append(opts, client.OptHeader("x-goog-api-key", apiKey))
	c, err := client.New(opts...)
	if err != nil {
		return nil, err
	}

	// Return the client
	return &Client{Client: c, ModelCache: agent.NewModelCache(time.Hour, 40)}, nil
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Name returns the name of the client
func (*Client) Name() string {
	return defaultName
}

// ToolOption implements llm.ToolOptioner by delegating to WithTool.
func (*Client) ToolOption(def schema.ToolDefinition) (opt.Opt, error) {
	return WithTool(def), nil
}
