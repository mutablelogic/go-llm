package httpclient

import (
	"context"
	"fmt"
	"strings"

	// Packages
	client "github.com/mutablelogic/go-client"
	opt "github.com/mutablelogic/go-llm/pkg/opt"
	schema "github.com/mutablelogic/go-llm/kernel/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Ask sends a stateless ask request and returns the final response.
// When streamFn is non-nil, the request is made as an SSE stream and streamed
// delta events are dispatched to the callback before the final result is returned.
func (c *Client) Ask(ctx context.Context, req schema.AskRequest, streamFn opt.StreamFn) (*schema.AskResponse, error) {
	if req.Provider != nil {
		req.Provider = types.Ptr(strings.TrimSpace(*req.Provider))
	}
	if req.Model != nil {
		req.Model = types.Ptr(strings.TrimSpace(*req.Model))
	}
	req.Text = strings.TrimSpace(req.Text)
	if req.Model == nil || *req.Model == "" {
		return nil, fmt.Errorf("model name cannot be empty")
	}
	if req.Text == "" {
		return nil, fmt.Errorf("text cannot be empty")
	}

	if streamFn != nil {
		return c.askStream(ctx, req, streamFn)
	}
	return c.askJSON(ctx, req)
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

func (c *Client) askJSON(ctx context.Context, req schema.AskRequest) (*schema.AskResponse, error) {
	httpReq, err := client.NewJSONRequest(req)
	if err != nil {
		return nil, err
	}

	var response schema.AskResponse
	if err := c.DoWithContext(ctx, httpReq, &response, client.OptPath("ask")); err != nil {
		return nil, err
	}

	return &response, nil
}

func (c *Client) askStream(ctx context.Context, req schema.AskRequest, streamFn opt.StreamFn) (*schema.AskResponse, error) {
	httpReq, err := client.NewJSONRequest(req)
	if err != nil {
		return nil, err
	}

	var response *schema.AskResponse
	var streamErr error

	callback := func(evt client.TextStreamEvent) error {
		switch evt.Event {
		case schema.EventAssistant, schema.EventThinking, schema.EventTool:
			var delta schema.StreamDelta
			if err := evt.Json(&delta); err != nil {
				return fmt.Errorf("malformed delta event: %w", err)
			}
			streamFn(delta.Role, delta.Text)
		case schema.EventError:
			var streamError schema.StreamError
			if err := evt.Json(&streamError); err != nil {
				return fmt.Errorf("malformed error event: %w", err)
			}
			streamErr = fmt.Errorf("%s", streamError.Error)
		case schema.EventResult:
			var askResponse schema.AskResponse
			if err := evt.Json(&askResponse); err != nil {
				return fmt.Errorf("malformed result event: %w", err)
			}
			response = &askResponse
		}
		return nil
	}

	var discard struct{}
	if err := c.DoWithContext(ctx, httpReq, &discard,
		client.OptPath("ask"),
		client.OptReqHeader("Accept", "text/event-stream"),
		client.OptTextStreamCallback(callback),
		client.OptNoTimeout(),
	); err != nil {
		return nil, err
	}
	if streamErr != nil {
		return nil, streamErr
	}
	if response == nil {
		return nil, fmt.Errorf("no result event received in stream")
	}

	return response, nil
}
