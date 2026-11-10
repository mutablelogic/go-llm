/*
httpclient implements an API client for DeepSeek's web search and fetch
https://api-docs.deepseek.com/guides/search/
*/
package httpclient

import (
	"context"

	// Packages
	client "github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
)

////////////////////////////////////////////////////////////////////////////////
// TYPES

type Client struct {
	*client.Client
}

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	endPoint = "https://api.deepseek.com/v1/tools"
)

////////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// New returns a client for DeepSeek's web search and fetch with the API key,
// which is the same key as its models'. The endpoint can be replaced with
// client.OptEndpoint, for example for testing.
func New(apiKey string, opts ...client.ClientOpt) (*Client, error) {
	// Check for missing API key
	if apiKey == "" {
		return nil, llm.ErrBadParameter.With("missing API key")
	}

	// Create client, with the default endpoint first so it can be replaced,
	// and the API key as a bearer token
	opts = append([]client.ClientOpt{client.OptEndpoint(endPoint)}, opts...)
	opts = append(opts, client.OptReqToken(client.Token{Scheme: client.Bearer, Value: apiKey}))
	client, err := client.New(opts...)
	if err != nil {
		return nil, err
	}

	// Return the client
	return &Client{Client: client}, nil
}

////////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Search returns the results of searching the web for the query in the
// request
func (c *Client) Search(ctx context.Context, req *SearchRequest) (SearchResponse, error) {
	var response SearchResponse
	payload, err := client.NewJSONRequest(req)
	if err != nil {
		return SearchResponse{}, err
	}
	if err := c.DoWithContext(ctx, payload, &response, client.OptPath("search")); err != nil {
		return SearchResponse{}, err
	}
	return response, nil
}

// Fetch returns the title, and the content as Markdown, of the web page with
// the URL in the request
func (c *Client) Fetch(ctx context.Context, req *FetchRequest) (FetchResponse, error) {
	var response FetchResponse
	payload, err := client.NewJSONRequest(req)
	if err != nil {
		return FetchResponse{}, err
	}
	if err := c.DoWithContext(ctx, payload, &response, client.OptPath("fetch")); err != nil {
		return FetchResponse{}, err
	}
	return response, nil
}
