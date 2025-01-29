/*
anthropic implements an API client for anthropic (https://docs.anthropic.com/en/api/getting-started)
*/
package anthropic

import (
	"context"

	// Packages
	client "github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type Client struct {
	*client.Client
}

var _ llm.Agent = (*Client)(nil)

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	endPoint            = "https://api.anthropic.com/v1"
	defaultVersion      = "2023-06-01"
	defaultName         = "anthropic"
	defaultMessageModel = "claude-3-haiku-20240307"
	defaultMaxTokens    = 1024
)

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// Create a new client
func New(ApiKey string, opts ...client.ClientOpt) (*Client, error) {
	// Create client
	opts = append(opts, client.OptEndpoint(endPoint))
	opts = append(opts, client.OptHeader("x-api-key", ApiKey), client.OptHeader("anthropic-version", defaultVersion))
	client, err := client.New(opts...)
	if err != nil {
		return nil, err
	}

	// Return the client
	return &Client{client}, nil
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Return the name of the agent
func (*Client) Name() string {
	return defaultName
}

// Generate a response from a prompt
func (*Client) Generate(context.Context, llm.Model, llm.Context, ...llm.Opt) (*llm.Response, error) {
	return nil, llm.ErrNotImplemented
}

// Embedding vector generation
func (*Client) Embedding(context.Context, llm.Model, string, ...llm.Opt) ([]float64, error) {
	return nil, llm.ErrNotImplemented
}

// Create user message context
func (*Client) UserPrompt(string, ...llm.Opt) llm.Context {
	return nil
}

// Create the result of calling a tool
func (*Client) ToolResult(any) llm.Context {
	return nil
}
