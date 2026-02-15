package httpclient

import (
	"context"
	"fmt"

	// Packages
	client "github.com/mutablelogic/go-client"
	opt "github.com/mutablelogic/go-llm/pkg/opt"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// ListTools returns a list of all available tools.
// Use WithLimit and WithOffset to paginate results.
func (c *Client) ListTools(ctx context.Context, opts ...opt.Opt) (*schema.ListToolResponse, error) {
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
	var response schema.ListToolResponse
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
