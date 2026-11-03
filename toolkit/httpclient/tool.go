package httpclient

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"

	// Packages
	client "github.com/mutablelogic/go-client"
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
