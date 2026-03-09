package client

import (
	"context"
	"fmt"
	"sync"

	// Packages
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	client "github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// ClientOpt → see opt.go (type Opt func(*Client) error)

// Client is an MCP HTTP client that wraps the base HTTP client
// and provides typed methods for interacting with the MCP server.
type Client struct {
	*client.Client
	sdkmcp.Implementation

	// MCP session state
	url     string
	authFn  func(context.Context, string) error
	session *sdkmcp.ClientSession
	mu      sync.Mutex

	// go-client opts accumulated by WithClientOpt during New(), consumed once.
	goOpts []client.ClientOpt

	// Cached lists, refreshed after connect and on change notifications.
	tools     []llm.Tool
	prompts   []llm.Prompt
	resources []llm.Resource

	// Notification handlers (nil = ignore)
	onLoggingMessage      OnLoggingMessage
	onProgress            OnProgress
	onToolListChanged     OnToolListChanged
	onPromptListChanged   OnPromptListChanged
	onResourceListChanged OnResourceListChanged
	onResourceUpdated     OnResourceUpdated
}

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// New creates a new HTTP client with the given base URL and options.
// The url parameter should point to the MCP server endpoint, e.g.
// https://mcp.asana.com/sse
//
// authFn is called when the server returns 401 to perform the OAuth flow.
// Pass nil to disable auth.
func New(url, name, version string, opts ...Opt) (*Client, error) {
	c := new(Client)

	// Check parameters.
	if url == "" {
		return nil, fmt.Errorf("url is required")
	} else if name == "" {
		return nil, fmt.Errorf("name is required")
	}

	// Apply mcp opts first so WithClientOpt entries are collected into c.goOpts.
	for _, opt := range opts {
		if err := opt(c); err != nil {
			return nil, err
		}
	}

	// client.New() unconditionally installs a transport.NewToken middleware
	// that calls client.AccessToken() on every request. AccessToken() reads
	// from an atomicToken field that the OAuthFlow updates via setToken() when
	// an authorization flow completes. connect() shallow-copies *http.Client,
	// which keeps the same transport chain and therefore the same token closure,
	// so Bearer tokens are injected automatically after auth without requiring
	// any extra option or nil-guard.
	if cl, err := client.New(append(c.goOpts, client.OptEndpoint(url))...); err != nil {
		return nil, err
	} else {
		c.Client = cl
		c.Implementation = sdkmcp.Implementation{Name: name, Version: version}
		c.url = url
		c.goOpts = nil
	}

	// Return the client; caller should call Run to connect and drive the session.
	return c, nil
}
