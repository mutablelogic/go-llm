package anthropic

import (
	"context"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	opt "github.com/mutablelogic/go-llm/pkg/opt"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	types "github.com/mutablelogic/go-llm/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// MESSENGER INTERFACE IMPLEMENTATION

var _ llm.Messenger = (*Client)(nil)

// Send sends a single message and returns the response (stateless)
func (c *Client) WithoutSession(ctx context.Context, model schema.Model, message *schema.Message, opts ...opt.Opt) (*schema.Message, error) {
	if message == nil {
		return nil, llm.ErrBadParameter.With("WithSession requires a non-nil message")
	}
	if model.OwnedBy != c.Name() {
		return nil, llm.ErrBadParameter.With("model does not belong to this client")
	}

	// Create a new session with the single message
	session := schema.Session{message}

	// Use the Messages API to send the message
	response, err := c.Messages(ctx, model.Name, &session, opts...)
	if err != nil {
		return nil, err
	}

	// Return the response directly from Messages()
	return response, nil
}

// WithSession sends a message within a session and returns the response (stateful)
func (c *Client) WithSession(ctx context.Context, model schema.Model, session *schema.Session, message *schema.Message, opts ...opt.Opt) (*schema.Message, error) {
	if session == nil {
		return nil, llm.ErrBadParameter.With("WithSession requires a non-nil session")
	}
	if message == nil {
		return nil, llm.ErrBadParameter.With("WithSession requires a non-nil message")
	}
	if model.OwnedBy != c.Name() {
		return nil, llm.ErrBadParameter.With("model does not belong to this client")
	}

	// Append the message to the session
	session.Append(types.Val(message))

	// Use the Messages API to generate a response
	response, err := c.Messages(ctx, model.Name, session, opts...)
	if err != nil {
		return nil, err
	}

	// Return the response directly from Messages()
	return response, nil
}
