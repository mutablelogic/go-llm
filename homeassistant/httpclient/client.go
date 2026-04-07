/*
homeassistant implements an API client for Home Assistant API
https://developers.home-assistant.io/docs/api/rest/
*/
package homeassistant

import (
	// Packages
	"github.com/mutablelogic/go-client"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type Client struct {
	*client.Client
}

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

func New(endPoint, apiKey string, opts ...client.ClientOpt) (*Client, error) {
	// Add a final slash to the endpoint
	if len(endPoint) > 0 && endPoint[len(endPoint)-1] != '/' {
		endPoint += "/"
	}

	// Create client
	client, err := client.New(append(opts, client.OptEndpoint(endPoint), client.OptReqToken(client.Token{
		Scheme: client.Bearer,
		Value:  apiKey,
	}))...)
	if err != nil {
		return nil, err
	}

	// Return the client
	return &Client{client}, nil
}
