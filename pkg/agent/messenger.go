package agent

import (
	"context"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	opt "github.com/mutablelogic/go-llm/pkg/opt"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS - Messenger

// Send sends a single message and returns the response (stateless)
func (a *agent) Send(ctx context.Context, model schema.Model, message *schema.Message, opts ...opt.Opt) (*schema.Message, error) {
	// Get the client for this model
	client := a.clientForModel(model)
	if client == nil {
		return nil, llm.ErrNotFound.Withf("no client found for model: %s", model.Name)
	}

	// Check if client implements Messenger
	messenger, ok := client.(llm.Messenger)
	if !ok {
		return nil, llm.ErrNotImplemented.Withf("client %q does not support messaging", client.Name())
	}

	// Send the message
	return messenger.Send(ctx, model, message, opts...)
}

// WithSession sends a message within a session and returns the response (stateful)
func (a *agent) WithSession(ctx context.Context, model schema.Model, session *schema.Session, message *schema.Message, opts ...opt.Opt) (*schema.Message, error) {
	// Get the client for this model
	client := a.clientForModel(model)
	if client == nil {
		return nil, llm.ErrNotFound.Withf("no client found for model: %s", model.Name)
	}

	// Check if client implements Messenger
	messenger, ok := client.(llm.Messenger)
	if !ok {
		return nil, llm.ErrNotImplemented.Withf("client %q does not support messaging", client.Name())
	}

	// Send the message within the session
	return messenger.WithSession(ctx, model, session, message, opts...)
}
