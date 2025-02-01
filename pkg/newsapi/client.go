/*
newsapi implements an API client for NewsAPI (https://newsapi.org/docs)
*/
package newsapi

import (
	// Packages
	"github.com/mutablelogic/go-client"
	"github.com/mutablelogic/go-llm/pkg/tool"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type Client struct {
	*client.Client
}

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	endPoint = "https://newsapi.org/v2"
)

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

func New(ApiKey string, opts ...client.ClientOpt) (*Client, error) {
	// Create client
	client, err := client.New(append(opts, client.OptEndpoint(endPoint), client.OptHeader("X-Api-Key", ApiKey))...)
	if err != nil {
		return nil, err
	}

	// Return the client
	return &Client{client}, nil
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

func (newsapi *Client) RegisterWithToolKit(toolkit *tool.ToolKit) error {
	// Register tools
	if err := toolkit.Register(&headlines{newsapi, ""}); err != nil {
		return err
	}

	// Return success
	return nil
}
