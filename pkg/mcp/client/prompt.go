package client

import (
	"context"
	"encoding/json"

	// Packages
	mcp "github.com/mutablelogic/go-llm/pkg/mcp/schema"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// GetPrompt retrieves a prompt by name from the MCP server, optionally
// providing arguments to customize the prompt template.
func (c *Client) GetPrompt(ctx context.Context, name string, args map[string]string) (*mcp.ResponseGetPrompt, error) {
	if err := c.init(ctx); err != nil {
		return nil, err
	}

	// Marshal request parameters
	params, err := json.Marshal(mcp.RequestGetPrompt{
		Name:      name,
		Arguments: args,
	})
	if err != nil {
		return nil, err
	}

	req := mcp.Request{
		Version: mcp.RPCVersion,
		Method:  mcp.MessageTypeGetPrompt,
		ID:      c.nextId(),
		Payload: json.RawMessage(params),
	}

	resp, err := c.doRPC(ctx, req)
	if err != nil {
		return nil, err
	}

	// Decode result
	var result mcp.ResponseGetPrompt
	if err := decodeResult(resp.Result, &result); err != nil {
		return nil, err
	}

	return &result, nil
}
