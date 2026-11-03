package httpclient

import (
	"context"
	"errors"
	"fmt"

	// Packages
	client "github.com/mutablelogic/go-client"
	schema "github.com/mutablelogic/go-llm/registry/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// doStream sends the request to the path, accepting server-sent events, and
// calls the function with each event, until the result event, which is
// returned, or an error event, which is the error. Returns an error when
// the stream ends without either. The request isn't timed out, but is
// cancelled with the context.
func doStream[T any](ctx context.Context, c *Client, body client.Payload, fn func(client.TextStreamEvent) error, path ...any) (*T, error) {
	var result *T
	var resultErr error
	callback := func(event client.TextStreamEvent) error {
		switch event.Event {
		case schema.EventError:
			var data schema.ErrorEvent
			if err := event.Json(&data); err != nil {
				return fmt.Errorf("malformed error event: %w", err)
			}
			resultErr = errors.New(data.Error)
		case schema.EventResult:
			var data T
			if err := event.Json(&data); err != nil {
				return fmt.Errorf("malformed result event: %w", err)
			}
			result = &data
		default:
			return fn(event)
		}
		return nil
	}
	if err := c.DoWithContext(ctx, body, nil,
		client.OptPath(path...),
		client.OptReqHeader("Accept", types.ContentTypeTextStream),
		client.OptTextStreamCallback(callback),
		client.OptNoTimeout(),
	); err != nil {
		return nil, err
	}
	switch {
	case resultErr != nil:
		return nil, resultErr
	case result == nil:
		return nil, errors.New("the stream ended without a result")
	default:
		return result, nil
	}
}
