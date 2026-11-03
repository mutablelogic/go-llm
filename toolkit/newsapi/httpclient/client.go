/*
httpclient implements an API client for NewsAPI
https://newsapi.org/docs
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
	endPoint = "https://newsapi.org/v2"
)

////////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// New returns a client for NewsAPI with the API key. The endpoint can be
// replaced with client.OptEndpoint, for example for testing.
func New(apiKey string, opts ...client.ClientOpt) (*Client, error) {
	// Check for missing API key
	if apiKey == "" {
		return nil, llm.ErrBadParameter.With("missing API key")
	}

	// Create client, with the default endpoint first so it can be replaced,
	// and the API key in a header
	opts = append([]client.ClientOpt{client.OptEndpoint(endPoint)}, opts...)
	opts = append(opts, client.OptHeader("X-Api-Key", apiKey))
	client, err := client.New(opts...)
	if err != nil {
		return nil, err
	}

	// Return the client
	return &Client{Client: client}, nil
}

////////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Headlines returns the top headlines matching the request
func (c *Client) Headlines(ctx context.Context, req *HeadlinesRequest) ([]Article, error) {
	var response respArticles
	if err := c.DoWithContext(ctx, nil, &response, client.OptPath("top-headlines"), client.OptQuery(req.Values())); err != nil {
		return nil, err
	} else if err := response.err(); err != nil {
		return nil, err
	}
	return response.Articles, nil
}

// Articles returns the articles matching the request
func (c *Client) Articles(ctx context.Context, req *ArticlesRequest) ([]Article, error) {
	var response respArticles
	if err := c.DoWithContext(ctx, nil, &response, client.OptPath("everything"), client.OptQuery(req.Values())); err != nil {
		return nil, err
	} else if err := response.err(); err != nil {
		return nil, err
	}
	return response.Articles, nil
}

// Sources returns the news sources matching the request
func (c *Client) Sources(ctx context.Context, req *SourcesRequest) ([]Source, error) {
	var response respSources
	if err := c.DoWithContext(ctx, nil, &response, client.OptPath("top-headlines", "sources"), client.OptQuery(req.Values())); err != nil {
		return nil, err
	} else if err := response.err(); err != nil {
		return nil, err
	}
	return response.Sources, nil
}
