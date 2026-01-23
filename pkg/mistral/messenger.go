package mistral

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
func (c *Client) Send(ctx context.Context, model schema.Model, message *schema.Message, opts ...opt.Opt) (*schema.Message, error) {
	session := schema.Session{message}
	return c.Chat(ctx, model.Name, &session, opts...)
}

// WithSession sends a message within a session and returns the response (stateful)
func (c *Client) WithSession(ctx context.Context, model schema.Model, session *schema.Session, message *schema.Message, opts ...opt.Opt) (*schema.Message, error) {
	session.Append(*message)
	return c.Chat(ctx, model.Name, session, opts...)
}
