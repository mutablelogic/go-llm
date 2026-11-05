package httpclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	// Packages
	client "github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/toolkit/schema"
	httpresponse "github.com/mutablelogic/go-server/pkg/httpresponse"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// ListTools returns the tools matching the request, in order of canonical
// name, with the total number matching
func (c *Client) ListTools(ctx context.Context, req schema.ToolListRequest) (*schema.ToolList, error) {
	var response schema.ToolList
	if err := c.DoWithContext(ctx, client.MethodGet, &response, client.OptPath("llm", "tool"), client.OptQuery(req.Query())); err != nil {
		return nil, err
	}

	// Return the response
	return types.Ptr(response), nil
}

// GetTool returns the tool with the name, which is its canonical name, such
// as "weatherapi:current", or its name without a namespace, when only one
// namespace has a tool with the name
func (c *Client) GetTool(ctx context.Context, name string) (*schema.Tool, error) {
	var response schema.Tool
	if err := c.DoWithContext(ctx, client.MethodGet, &response, client.OptPath("llm", "tool", name)); err != nil {
		return nil, err
	}

	// Return the response
	return types.Ptr(response), nil
}

// CallTool runs the tool with the canonical name, such as "weatherapi:current",
// with the input, which may be empty, and returns its result as JSON, or nil
// when the tool has no result
func (c *Client) CallTool(ctx context.Context, name string, input json.RawMessage) (json.RawMessage, error) {
	req, err := client.NewJSONRequest(schema.CallToolRequest{Input: input})
	if err != nil {
		return nil, err
	}

	// Perform request
	var response toolResult
	if err := c.DoWithContext(ctx, req, &response, client.OptPath("llm", "tool", name)); err != nil {
		return nil, err
	}

	// Return the result, which is nil when the tool has no result
	return response.result, nil
}

// CallToolStream runs the tool with the canonical name, with the input, as
// CallTool does, and calls the function with what it logs, and its
// progress, as llm.Progress, as JSON, as it runs, and returns its result.
// The request isn't timed out, as a tool, such as an agent, can take a
// while, but is cancelled with the context.
func (c *Client) CallToolStream(ctx context.Context, name string, input json.RawMessage, stream llm.GeneratorStreamFn) (json.RawMessage, error) {
	if stream == nil {
		return c.CallTool(ctx, name, input)
	}
	req, err := client.NewJSONRequestEx(http.MethodPost, schema.CallToolRequest{Input: input}, types.ContentTypeTextStream)
	if err != nil {
		return nil, err
	}

	// The events, until the result, or the error
	var result json.RawMessage
	var resultErr error
	ended := false
	callback := func(event client.TextStreamEvent) error {
		switch event.Event {
		case schema.EventLog:
			var data schema.LogEvent
			if err := event.Json(&data); err != nil {
				return fmt.Errorf("malformed %s event: %w", event.Event, err)
			}
			stream(llm.StreamLog, data.Text)
		case schema.EventProgress:
			var data schema.ProgressEvent
			if err := event.Json(&data); err != nil {
				return fmt.Errorf("malformed %s event: %w", event.Event, err)
			}
			message := data.Message
			if message == "" {
				message = data.Status
			}
			stream(llm.StreamProgress, llm.Progress{Progress: data.Progress, Total: data.Total, Message: message}.Text())
		case schema.EventResult:
			// The result is left out when there's none, so JSON null is a
			// result
			var data map[string]json.RawMessage
			if err := event.Json(&data); err != nil {
				return fmt.Errorf("malformed %s event: %w", event.Event, err)
			}
			result, ended = data["result"], true
		case schema.EventError:
			var data schema.ErrorEvent
			if err := event.Json(&data); err != nil {
				return fmt.Errorf("malformed %s event: %w", event.Event, err)
			}
			resultErr, ended = errors.New(data.Error), true
		}
		return nil
	}
	if err := c.DoWithContext(ctx, req, nil,
		client.OptPath("llm", "tool", name),
		client.OptReqHeader("Accept", types.ContentTypeTextStream),
		client.OptTextStreamCallback(callback),
		client.OptNoTimeout(),
	); err != nil {
		return nil, err
	} else if resultErr != nil {
		return nil, resultErr
	} else if !ended {
		return nil, errors.New("the stream ended without a result")
	}
	return result, nil
}

///////////////////////////////////////////////////////////////////////////////
// TYPES

// toolResult is the result of a tool, which is read from the response body,
// as a tool without a result returns no content, and no content type
type toolResult struct {
	result json.RawMessage
}

var _ client.Unmarshaler = (*toolResult)(nil)

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// Unmarshal reads the result of the tool from the response body, which must
// be JSON, or empty when the tool has no result
func (t *toolResult) Unmarshal(_ http.Header, r io.Reader) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	if data = bytes.TrimSpace(data); len(data) == 0 {
		t.result = nil
	} else if !json.Valid(data) {
		return httpresponse.ErrInternalError.With("tool result isn't valid JSON")
	} else {
		t.result = data
	}
	return nil
}
