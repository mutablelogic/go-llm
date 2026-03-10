package client

import (
	"context"

	// Packages
	client "github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Opt is a functional option for configuring a Client.
type Opt func(*Client) error

// MetaValue is a single key/value pair for the MCP _meta field.
type MetaValue struct {
	Key   string
	Value any
}

// Meta returns a MetaValue that can be passed to CallTool to populate the
// protocol-level _meta object sent with the request.
func Meta(key string, value any) MetaValue {
	return MetaValue{Key: key, Value: value}
}

// OnLoggingMessage is called when the server sends a logging message.
type OnLoggingMessage func(ctx context.Context, level, logger string, data any)

// OnProgress is called when the server sends a progress notification.
type OnProgress func(ctx context.Context, token any, progress, total float64, message string)

// OnToolListChanged is called when the server sends a tool-list-changed notification.
type OnToolListChanged func(ctx context.Context)

// OnPromptListChanged is called when the server sends a prompt-list-changed notification.
type OnPromptListChanged func(ctx context.Context)

// OnResourceListChanged is called when the server sends a resource-list-changed notification.
type OnResourceListChanged func(ctx context.Context)

// OnResourceUpdated is called when the server sends a resource-updated notification.
type OnResourceUpdated func(ctx context.Context, resource llm.Resource)

///////////////////////////////////////////////////////////////////////////////
// CLIENT OPTIONS

// WithAuth registers a function called when the server returns 401 to perform
// the OAuth discovery and authorization flow. Pass nil to disable auth.
func WithAuth(fn func(context.Context, string) error) Opt {
	return func(c *Client) error {
		c.authFn = fn
		return nil
	}
}

// WithClientOpt wraps one or more go-client options so they can be passed
// alongside mcp Opt values to New().
func WithClientOpt(goOpts ...client.ClientOpt) Opt {
	return func(c *Client) error {
		c.goOpts = append(c.goOpts, goOpts...)
		return nil
	}
}

// OptOnLoggingMessage registers a callback invoked whenever the server sends
// a logging message notification.
func OptOnLoggingMessage(fn OnLoggingMessage) Opt {
	return func(c *Client) error {
		c.onLoggingMessage = fn
		return nil
	}
}

// OptOnProgress registers a callback invoked whenever the server sends a
// progress notification.
func OptOnProgress(fn OnProgress) Opt {
	return func(c *Client) error {
		c.onProgress = fn
		return nil
	}
}

// OptOnToolListChanged registers a callback invoked whenever the server
// notifies the client that its tool list has changed.
func OptOnToolListChanged(fn OnToolListChanged) Opt {
	return func(c *Client) error {
		c.onToolListChanged = fn
		return nil
	}
}

// OptOnPromptListChanged registers a callback invoked with the refreshed
// prompt list whenever the server notifies the client that its prompt list
// has changed.
func OptOnPromptListChanged(fn OnPromptListChanged) Opt {
	return func(c *Client) error {
		c.onPromptListChanged = fn
		return nil
	}
}

// OptOnResourceListChanged registers a callback invoked with the refreshed
// resource list whenever the server notifies the client that its resource
// list has changed.
func OptOnResourceListChanged(fn OnResourceListChanged) Opt {
	return func(c *Client) error {
		c.onResourceListChanged = fn
		return nil
	}
}

// OptOnResourceUpdated registers a callback invoked with the URI of the
// resource whenever the server sends a resource-updated notification.
func OptOnResourceUpdated(fn OnResourceUpdated) Opt {
	return func(c *Client) error {
		c.onResourceUpdated = fn
		return nil
	}
}
