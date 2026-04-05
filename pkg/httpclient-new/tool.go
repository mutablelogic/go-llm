package httpclient

import (
	"context"
	"fmt"

	// Packages
	client "github.com/mutablelogic/go-client"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// ListTools returns tools matching the given request parameters.
func (c *Client) ListTools(ctx context.Context, req schema.ToolListRequest) (*schema.ToolList, error) {
	var response schema.ToolList
	if err := c.DoWithContext(ctx, client.MethodGet, &response, client.OptPath("tool"), client.OptQuery(req.Query())); err != nil {
		return nil, err
	}

	return &response, nil
}

// GetTool returns metadata for a specific tool by name.
func (c *Client) GetTool(ctx context.Context, name string) (*schema.ToolMeta, error) {
	if name == "" {
		return nil, fmt.Errorf("tool name cannot be empty")
	}

	var response schema.ToolMeta
	if err := c.DoWithContext(ctx, client.MethodGet, &response, client.OptPath("tool", name)); err != nil {
		return nil, err
	}

	return &response, nil
}
