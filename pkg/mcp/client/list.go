package client

import (
	"context"
	"encoding/json"

	// Packages
	mcp "github.com/mutablelogic/go-llm/pkg/mcp/schema"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// ListTools returns the tools available on the MCP server.
func (c *Client) ListTools(ctx context.Context) ([]*mcp.Tool, error) {
	if err := c.init(ctx); err != nil {
		return nil, err
	}

	var result []*mcp.Tool
	var cursor string
	for {
		// Build the request with optional cursor
		var params json.RawMessage
		if cursor != "" {
			data, err := json.Marshal(mcp.RequestList{Cursor: cursor})
			if err != nil {
				return nil, err
			}
			params = data
		}

		req := mcp.Request{
			Version: mcp.RPCVersion,
			Method:  mcp.MessageTypeListTools,
			ID:      c.nextId(),
			Payload: params,
		}

		resp, err := c.doRPC(ctx, req)
		if err != nil {
			return nil, err
		}

		// Decode result
		var listResp mcp.ResponseListTools
		if err := decodeResult(resp.Result, &listResp); err != nil {
			return nil, err
		}

		result = append(result, listResp.Tools...)

		// Check for next page
		if listResp.NextCursor == "" {
			break
		}
		cursor = listResp.NextCursor
	}

	// Cache tools by name for validation in CallTool
	c.mu.Lock()
	c.tools = make(map[string]*mcp.Tool, len(result))
	for _, t := range result {
		c.tools[t.Name] = t
	}
	c.mu.Unlock()

	return result, nil
}

// ListPrompts returns the prompts available on the MCP server.
func (c *Client) ListPrompts(ctx context.Context) ([]*mcp.Prompt, error) {
	if err := c.init(ctx); err != nil {
		return nil, err
	}

	var result []*mcp.Prompt
	var cursor string
	for {
		var params json.RawMessage
		if cursor != "" {
			data, err := json.Marshal(mcp.RequestList{Cursor: cursor})
			if err != nil {
				return nil, err
			}
			params = data
		}

		req := mcp.Request{
			Version: mcp.RPCVersion,
			Method:  mcp.MessageTypeListPrompts,
			ID:      c.nextId(),
			Payload: params,
		}

		resp, err := c.doRPC(ctx, req)
		if err != nil {
			return nil, err
		}

		var listResp mcp.ResponseListPrompts
		if err := decodeResult(resp.Result, &listResp); err != nil {
			return nil, err
		}

		result = append(result, listResp.Prompts...)

		if listResp.NextCursor == "" {
			break
		}
		cursor = listResp.NextCursor
	}

	return result, nil
}

// ListResources returns the resources available on the MCP server.
func (c *Client) ListResources(ctx context.Context) ([]any, error) {
	if err := c.init(ctx); err != nil {
		return nil, err
	}

	var result []any
	var cursor string
	for {
		var params json.RawMessage
		if cursor != "" {
			data, err := json.Marshal(mcp.RequestList{Cursor: cursor})
			if err != nil {
				return nil, err
			}
			params = data
		}

		req := mcp.Request{
			Version: mcp.RPCVersion,
			Method:  mcp.MessageTypeListResources,
			ID:      c.nextId(),
			Payload: params,
		}

		resp, err := c.doRPC(ctx, req)
		if err != nil {
			return nil, err
		}

		var listResp mcp.ResponseListResources
		if err := decodeResult(resp.Result, &listResp); err != nil {
			return nil, err
		}

		result = append(result, listResp.Resources...)

		if listResp.NextCursor == "" {
			break
		}
		cursor = listResp.NextCursor
	}

	return result, nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// decodeResult marshals resp.Result (any) back to JSON and decodes into dest.
func decodeResult(result any, dest any) error {
	data, err := json.Marshal(result)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, dest)
}
