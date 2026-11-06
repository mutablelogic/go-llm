package httpclient

import (
	"context"
	"fmt"

	// Packages
	client "github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/broadcaster/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// WatchEvents calls the function with each of the broadcaster's events, as
// the server streams them, such as when a provider, or a session, is
// created, updated or deleted, until the context is cancelled, when it returns nil, or the
// function returns an error, which it returns, or the stream ends. When the
// server stops, it returns ErrServiceUnavailable.
func (c *Client) WatchEvents(ctx context.Context, fn func(schema.Event) error) error {
	if fn == nil {
		return llm.ErrBadParameter.With("missing function")
	}

	// Read the events, and ignore the pings. The function's error is kept,
	// as it's returned even when the context is cancelled as well.
	var fnErr, shutdown error
	callback := func(event client.TextStreamEvent) error {
		if event.Event == schema.EventShutdown {
			shutdown = llm.ErrServiceUnavailable.With("server is stopping")
			return nil
		} else if event.Event != schema.EventName {
			return nil
		}
		var data schema.Event
		if err := event.Json(&data); err != nil {
			return fmt.Errorf("malformed %s event: %w", event.Event, err)
		}
		fnErr = fn(data)
		return fnErr
	}
	err := c.DoWithContext(ctx, client.MethodGet, nil,
		client.OptPath("events"),
		client.OptReqHeader("Accept", types.ContentTypeTextStream),
		client.OptTextStreamCallback(callback),
		client.OptNoTimeout(),
	)
	switch {
	case fnErr != nil:
		return fnErr
	case ctx.Err() != nil:
		return nil
	case shutdown != nil:
		return shutdown
	default:
		return err
	}
}
