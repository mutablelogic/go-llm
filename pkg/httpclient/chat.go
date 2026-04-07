package httpclient

import (
	"context"
	"fmt"
	"strings"

	// Packages
	uuid "github.com/google/uuid"
	client "github.com/mutablelogic/go-client"
	opt "github.com/mutablelogic/go-llm/pkg/opt"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Chat sends a stateful chat request and returns the final response.
// When streamFn is non-nil, the request is made as an SSE stream and streamed
// delta events are dispatched to the callback before the final result is returned.
func (c *Client) Chat(ctx context.Context, req schema.ChatRequest, streamFn opt.StreamFn) (*schema.ChatResponse, error) {
	if req.Session == uuid.Nil {
		return nil, fmt.Errorf("session ID cannot be nil")
	}
	req.Text = strings.TrimSpace(req.Text)
	req.SystemPrompt = strings.TrimSpace(req.SystemPrompt)
	if req.Text == "" {
		return nil, fmt.Errorf("text cannot be empty")
	}

	if streamFn != nil {
		return c.chatStream(ctx, req, streamFn)
	}
	return c.chatJSON(ctx, req)
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

func (c *Client) chatJSON(ctx context.Context, req schema.ChatRequest) (*schema.ChatResponse, error) {
	httpReq, err := client.NewJSONRequest(req)
	if err != nil {
		return nil, err
	}

	var response schema.ChatResponse
	if err := c.DoWithContext(ctx, httpReq, &response, client.OptPath("chat")); err != nil {
		return nil, err
	}

	return &response, nil
}

func (c *Client) chatStream(ctx context.Context, req schema.ChatRequest, streamFn opt.StreamFn) (*schema.ChatResponse, error) {
	httpReq, err := client.NewJSONRequest(req)
	if err != nil {
		return nil, err
	}

	var response *schema.ChatResponse
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
			var chatResponse schema.ChatResponse
			if err := evt.Json(&chatResponse); err != nil {
				return fmt.Errorf("malformed result event: %w", err)
			}
			response = &chatResponse
		}
		return nil
	}

	var discard struct{}
	if err := c.DoWithContext(ctx, httpReq, &discard,
		client.OptPath("chat"),
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
