package gemini

import (
	"context"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	"github.com/mutablelogic/go-llm/pkg/opt"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
)

///////////////////////////////////////////////////////////////////////////////
// MESSENGER INTERFACE

var _ llm.Messenger = (*Client)(nil)

// Send sends a single message and returns the response (stateless)
func (c *Client) WithoutSession(ctx context.Context, model schema.Model, message *schema.Message, opts ...opt.Opt) (*schema.Message, error) {
	// Create a new session with the single message
	session := schema.Session{message}

	// Use the existing Chat method
	response, err := c.Chat(ctx, model.Name, &session, opts...)
	if err != nil {
		return nil, err
	}

	return response, nil
}

// WithSession sends a message within a session and returns the response (stateful)
func (c *Client) WithSession(ctx context.Context, model schema.Model, session *schema.Session, message *schema.Message, opts ...opt.Opt) (*schema.Message, error) {
	// Append the new message to the session
	session.Append(*message)

	// Use the existing Chat method
	return c.Chat(ctx, model.Name, session, opts...)
}
