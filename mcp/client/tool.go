package client

import (
	"context"
	"encoding/json"

	// Packages
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	llm "github.com/mutablelogic/go-llm"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Instructions returns how to use the server, and its tools, as it said
// when it was connected, or nothing, when it isn't, or said nothing
func (c *Client) Instructions() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.session == nil {
		return ""
	} else if result := c.session.InitializeResult(); result != nil {
		return result.Instructions
	}
	return ""
}

// ListTools returns the server's tools, every page of them. Returns
// ErrServiceUnavailable when it isn't connected.
func (c *Client) ListTools(ctx context.Context) ([]*sdkmcp.Tool, error) {
	session, err := c.connected()
	if err != nil {
		return nil, err
	}
	var result []*sdkmcp.Tool
	for tool, err := range session.Tools(ctx, nil) {
		if err != nil {
			return nil, err
		}
		result = append(result, tool)
	}
	return result, nil
}

// CallTool calls the server's tool with the name, with its arguments, as
// JSON, which can be nil, and returns its result, which can be an error
// the tool returned, rather than one of the call. Returns
// ErrServiceUnavailable when it isn't connected.
func (c *Client) CallTool(ctx context.Context, name string, arguments json.RawMessage) (*sdkmcp.CallToolResult, error) {
	session, err := c.connected()
	if err != nil {
		return nil, err
	}

	// The arguments are sent as they are, as a json.RawMessage is encoded
	// as itself
	params := &sdkmcp.CallToolParams{Name: name}
	if len(arguments) > 0 {
		params.Arguments = arguments
	}
	return session.CallTool(ctx, params)
}

// ListPrompts returns the server's prompts, every page of them, or none,
// when it doesn't say it has any. Returns ErrServiceUnavailable when it
// isn't connected.
func (c *Client) ListPrompts(ctx context.Context) ([]*sdkmcp.Prompt, error) {
	session, err := c.connected()
	if err != nil {
		return nil, err
	}
	if result := session.InitializeResult(); result == nil || result.Capabilities == nil || result.Capabilities.Prompts == nil {
		return nil, nil
	}
	var result []*sdkmcp.Prompt
	for prompt, err := range session.Prompts(ctx, nil) {
		if err != nil {
			return nil, err
		}
		result = append(result, prompt)
	}
	return result, nil
}

// GetPrompt returns the messages of the server's prompt with the name, with
// its arguments. Returns ErrServiceUnavailable when it isn't connected.
func (c *Client) GetPrompt(ctx context.Context, name string, arguments map[string]string) (*sdkmcp.GetPromptResult, error) {
	session, err := c.connected()
	if err != nil {
		return nil, err
	}
	return session.GetPrompt(ctx, &sdkmcp.GetPromptParams{Name: name, Arguments: arguments})
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// connected returns the session, or ErrServiceUnavailable when it isn't
// connected
func (c *Client) connected() (*sdkmcp.ClientSession, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.session == nil {
		return nil, llm.ErrServiceUnavailable.Withf("%s isn't connected", c.endpoint)
	}
	return c.session, nil
}
