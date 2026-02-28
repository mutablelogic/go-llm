package client

import (
	"context"
	"encoding/json"
	"fmt"

	// Packages
	jsonschema "github.com/google/jsonschema-go/jsonschema"
	mcp "github.com/mutablelogic/go-llm/pkg/mcp/schema"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// CallTool executes a tool on the MCP server with the given name and arguments,
// and returns the tool result. It validates the tool name exists and the arguments
// match the tool's input schema before sending the request.
func (c *Client) CallTool(ctx context.Context, name string, args json.RawMessage) (*mcp.ResponseToolCall, error) {
	if err := c.init(ctx); err != nil {
		return nil, err
	}

	// Validate tool name and arguments against cached schema
	if err := c.validateToolCall(ctx, name, args); err != nil {
		return nil, err
	}

	// Marshal tool call parameters
	params, err := json.Marshal(mcp.RequestToolCall{
		Name:      name,
		Arguments: args,
	})
	if err != nil {
		return nil, err
	}

	req := mcp.Request{
		Version: mcp.RPCVersion,
		Method:  mcp.MessageTypeCallTool,
		ID:      c.nextId(),
		Payload: json.RawMessage(params),
	}

	resp, err := c.doRPC(ctx, req)
	if err != nil {
		return nil, err
	}

	// Decode result
	var result mcp.ResponseToolCall
	if err := decodeResult(resp.Result, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// validateToolCall validates that the tool exists and the arguments match
// the tool's input schema. If tools are not yet cached, it fetches them first.
func (c *Client) validateToolCall(ctx context.Context, name string, args json.RawMessage) error {
	// Fetch tools if not cached
	c.mu.Lock()
	if c.tools == nil {
		c.mu.Unlock()
		if _, err := c.ListTools(ctx); err != nil {
			return fmt.Errorf("failed to fetch tools: %w", err)
		}
		c.mu.Lock()
	}
	tool, ok := c.tools[name]
	c.mu.Unlock()

	// Check tool exists
	if !ok {
		return mcp.NewError(mcp.ErrorCodeMethodNotFound, fmt.Sprintf("tool not found: %q", name))
	}

	// Validate arguments against input schema if present
	if tool.InputSchema == nil {
		return nil
	}

	// Marshal the input schema to JSON, then parse with jsonschema
	schemaData, err := json.Marshal(tool.InputSchema)
	if err != nil {
		return fmt.Errorf("invalid input schema for tool %q: %w", name, err)
	}

	var schema jsonschema.Schema
	if err := json.Unmarshal(schemaData, &schema); err != nil {
		return fmt.Errorf("invalid input schema for tool %q: %w", name, err)
	}

	resolved, err := schema.Resolve(nil)
	if err != nil {
		return fmt.Errorf("invalid input schema for tool %q: %w", name, err)
	}

	// Unmarshal args into a native Go value for validation
	var argsValue any
	if len(args) > 0 {
		if err := json.Unmarshal(args, &argsValue); err != nil {
			return mcp.NewError(mcp.ErrorCodeInvalidParameters, fmt.Sprintf("invalid arguments JSON: %v", err))
		}
	} else {
		argsValue = map[string]any{}
	}

	// Validate against schema
	if err := resolved.Validate(argsValue); err != nil {
		return mcp.NewError(mcp.ErrorCodeInvalidParameters, fmt.Sprintf("argument validation failed: %v", err))
	}

	return nil
}
