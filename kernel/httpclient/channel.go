package httpclient

import (
	"context"
	"fmt"

	// Packages
	uuid "github.com/google/uuid"
	client "github.com/mutablelogic/go-client"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// ChannelCallback is the low-level callback used to drive the session channel.
// This is intentionally a thin wrapper over the underlying NDJSON stream so the
// channel can be used as a debug endpoint while the higher-level protocol is in flux.
type ChannelCallback func(context.Context, client.JSONStream) error

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Channel opens a bidirectional NDJSON stream for an existing session.
//
// The callback receives the raw stream so callers can send and receive frames
// directly while the session channel protocol is still evolving.
func (c *Client) Channel(ctx context.Context, session uuid.UUID, callback ChannelCallback) error {
	if session == uuid.Nil {
		return fmt.Errorf("session ID cannot be nil")
	}
	if callback == nil {
		return fmt.Errorf("channel callback cannot be nil")
	}

	return c.Stream(ctx, callback,
		client.OptPath("session", session.String(), "channel"),
		client.OptNoTimeout(),
	)
}
