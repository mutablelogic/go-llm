package httpclient

import (
	"context"
	"encoding/json"
	"fmt"

	// Packages
	client "github.com/mutablelogic/go-client"
	opt "github.com/mutablelogic/go-llm/pkg/opt"
	schema "github.com/mutablelogic/go-llm/kernel/schema"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// ListTools returns a list of all available tools.
// Use WithLimit and WithOffset to paginate results.
func (c *Client) ListTools(ctx context.Context, opts ...opt.Opt) (*schema.ToolList, error) {
	// Apply options
	o, err := opt.Apply(opts...)
	if err != nil {
		return nil, err
	}

	// Create request
	req := client.NewRequest()
	reqOpts := []client.RequestOpt{client.OptPath("tool")}
	if q := o.Query(opt.LimitKey, opt.OffsetKey); len(q) > 0 {
		reqOpts = append(reqOpts, client.OptQuery(q))
	}

	// Perform request
	var response schema.ToolList
	if err := c.DoWithContext(ctx, req, &response, reqOpts...); err != nil {
		return nil, err
	}

	// Return the response
	return &response, nil
}

// GetTool retrieves a specific tool by name.
func (c *Client) GetTool(ctx context.Context, name string) (*schema.ToolMeta, error) {
	if name == "" {
		return nil, fmt.Errorf("tool name cannot be empty")
	}

	// Create request
	req := client.NewRequest()
	reqOpts := []client.RequestOpt{client.OptPath("tool", name)}

	// Perform request
	var response schema.ToolMeta
	if err := c.DoWithContext(ctx, req, &response, reqOpts...); err != nil {
		return nil, err
	}

	// Return the response
	return &response, nil
}

// CallTool calls a tool by name with the given JSON input and returns the result.
func (c *Client) CallTool(ctx context.Context, name string, input json.RawMessage) (*schema.CallToolResponse, error) {
	if name == "" {
		return nil, fmt.Errorf("tool name cannot be empty")
	}

	payload, err := client.NewJSONRequest(schema.CallToolRequest{Input: input})
	if err != nil {
		return nil, err
	}

	var response schema.CallToolResponse
	if err := c.DoWithContext(ctx, payload, &response, client.OptPath("tool", name)); err != nil {
		return nil, err
	}
	return &response, nil
}
