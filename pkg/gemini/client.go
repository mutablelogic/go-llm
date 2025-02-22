/*
gemini implements an API client for Google's Gemini LLM
https://ai.google.dev/gemini-api/docs
*/
package gemini

import (

	// Packages
	client "github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type Client struct {
	*client.Client
	cache map[string]llm.Model
}

var _ llm.Agent = (*Client)(nil)

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	endPoint    = "https://generativelanguage.googleapis.com/v1beta"
	defaultName = "gemini"
)

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// Create a new client
func New(ApiKey string, opts ...client.ClientOpt) (*Client, error) {
	// Create client
	opts = append(opts, client.OptEndpoint(endPointWithKey(endPoint, ApiKey)))
	client, err := client.New(opts...)
	if err != nil {
		return nil, err
	}

	// Return the client
	return &Client{client, nil}, nil
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Return the name of the agent
func (Client) Name() string {
	return defaultName
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

func endPointWithKey(endpoint, key string) string {
	return endpoint + "?key=" + key
}
