/*
lmstudio implements an API client for LM Studio, which runs models on your
own computer. It replies, and embeds, with LM Studio's OpenAI-compatible API,
and lists, downloads, loads and unloads models with its own REST API, which
says what each model can do, and how much context it has.
https://lmstudio.ai/docs/developer/rest
*/
package lmstudio

import (
	"context"
	"net/url"
	"strings"

	// Packages
	client "github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
	openai "github.com/mutablelogic/go-llm/provider/openai"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Client is a client of an LM Studio server, with its own REST API, and the
// client of its OpenAI-compatible API, which replies, and embeds
type Client struct {
	*client.Client
	openai *openai.Client
	url    *url.URL
}

var _ llm.Provider = (*Client)(nil)

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

// DefaultEndpoint is the endpoint of LM Studio's server, on your own
// computer, unless it's told to listen on another port
const DefaultEndpoint = "http://localhost:1234/v1"

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// New returns a client of the LM Studio server at the endpoint, which is its
// OpenAI-compatible API's, such as "http://localhost:1234/v1", the default,
// or the server's, such as "localhost:1234", and an API token, which LM
// Studio needs only when it's told to, which is sent as a bearer token
func New(endPoint, apiKey string, opts ...client.ClientOpt) (*Client, error) {
	root, err := rootURL(endPoint)
	if err != nil {
		return nil, err
	}

	// The OpenAI-compatible API, which replies, and embeds
	compatible := root.JoinPath("v1")
	api, err := openai.New(compatible.String(), apiKey, opts...)
	if err != nil {
		return nil, err
	}

	// LM Studio's own API, which lists, downloads, loads and unloads models
	if apiKey != "" {
		opts = append(opts, client.OptReqToken(client.Token{Scheme: client.Bearer, Value: apiKey}))
	}
	native, err := client.New(append(opts, client.OptEndpoint(root.JoinPath("api", "v1").String()))...)
	if err != nil {
		return nil, err
	}
	return &Client{Client: native, openai: api, url: compatible}, nil
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Name returns the provider's name
func (*Client) Name() string {
	return llm.LMStudio
}

// URL returns the endpoint of the server's OpenAI-compatible API, such as
// "http://localhost:1234/v1"
func (c *Client) URL() *url.URL {
	u := *c.url
	return &u
}

// Self returns the underlying client implementation
func (c *Client) Self() llm.Provider {
	return c
}

// Ping checks the server can be reached, and that the API token, when it
// needs one, is accepted, by listing its models
func (c *Client) Ping(ctx context.Context) error {
	_, err := c.listModels(ctx)
	return err
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// rootURL returns the server's URL, without the path of its APIs, from an
// endpoint which is its OpenAI-compatible API's, such as
// "http://localhost:1234/v1", its own API's, such as
// "http://localhost:1234/api/v1", or the server's, such as "localhost:1234",
// which is http, as LM Studio's server is on your own computer, or the
// default, when it's empty
func rootURL(endPoint string) (*url.URL, error) {
	endPoint = strings.TrimSpace(endPoint)
	if endPoint == "" {
		endPoint = DefaultEndpoint
	}
	if !strings.Contains(endPoint, "://") {
		endPoint = "http://" + endPoint
	}
	u, err := url.Parse(endPoint)
	if err != nil {
		return nil, llm.ErrBadParameter.Withf("endpoint: %v", err)
	} else if u.Host == "" {
		return nil, llm.ErrBadParameter.Withf("endpoint %q has no host", endPoint)
	}
	path := strings.TrimSuffix(u.Path, "/")
	path = strings.TrimSuffix(path, "/v1")
	path = strings.TrimSuffix(path, "/api")
	u.Path, u.RawPath, u.RawQuery, u.Fragment = path, "", "", ""
	return u, nil
}
