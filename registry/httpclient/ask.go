package httpclient

import (
	"context"
	"errors"
	"fmt"
	"strings"

	// Packages
	client "github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/registry/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Ask returns the reply of the request's model to its messages, with the
// tools it called, for the caller to run, the reply's result, when the
// request has a schema, and the tokens used. When the stream function isn't
// nil, the reply is streamed, and the function is called with each piece of
// its text and thinking as it's generated. The request isn't timed out, as a
// model can take a while to reply, but is cancelled with the context.
func (c *Client) Ask(ctx context.Context, req schema.AskRequest, stream llm.GeneratorStreamFn) (*schema.AskResponse, error) {
	if strings.TrimSpace(req.Model) == "" {
		return nil, llm.ErrBadParameter.With("missing model")
	} else if len(req.Messages) == 0 {
		return nil, llm.ErrBadParameter.With("missing messages")
	}
	body, err := client.NewJSONRequest(req)
	if err != nil {
		return nil, err
	}

	// Without streaming, the reply
	if stream == nil {
		var response schema.AskResponse
		if err := c.DoWithContext(ctx, body, &response, client.OptPath("llm", "ask"), client.OptNoTimeout()); err != nil {
			return nil, err
		}
		return &response, nil
	}

	// With streaming, the pieces of the reply, and then the reply, or the
	// error
	var result *schema.AskResponse
	var resultErr error
	callback := func(event client.TextStreamEvent) error {
		switch event.Event {
		case schema.EventText, schema.EventThinking:
			var data schema.StreamEvent
			if err := event.Json(&data); err != nil {
				return fmt.Errorf("malformed %s event: %w", event.Event, err)
			}
			stream(llm.StreamType(event.Event), data.Text)
		case schema.EventError:
			var data schema.ErrorEvent
			if err := event.Json(&data); err != nil {
				return fmt.Errorf("malformed error event: %w", err)
			}
			resultErr = errors.New(data.Error)
		case schema.EventResult:
			var data schema.AskResponse
			if err := event.Json(&data); err != nil {
				return fmt.Errorf("malformed result event: %w", err)
			}
			result = &data
		}
		return nil
	}
	if err := c.DoWithContext(ctx, body, nil,
		client.OptPath("llm", "ask"),
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
		return nil, errors.New("the stream ended without a reply")
	default:
		return result, nil
	}
}
