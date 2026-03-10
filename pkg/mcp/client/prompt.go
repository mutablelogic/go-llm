package client

import (
	"context"

	// Packages
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	llm "github.com/mutablelogic/go-llm"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// mcpPrompt wraps an *sdkmcp.Prompt received from a server and implements
// llm.Prompt.
type mcpPrompt struct {
	p *sdkmcp.Prompt
}

// Ensure mcpPrompt implements llm.Prompt at compile time.
var _ llm.Prompt = (*mcpPrompt)(nil)

///////////////////////////////////////////////////////////////////////////////
// llm.Prompt INTERFACE

func (m *mcpPrompt) Name() string        { return m.p.Name }
func (m *mcpPrompt) Title() string       { return m.p.Title }
func (m *mcpPrompt) Description() string { return m.p.Description }

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// ListPrompts returns the cached list of prompts advertised by the connected
// MCP server. The cache is populated on connect and refreshed automatically
// on each PromptListChanged notification. Returns ErrNotConnected if not active.
func (c *Client) ListPrompts(_ context.Context) ([]llm.Prompt, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.session == nil {
		return nil, ErrNotConnected
	}
	return c.prompts, nil
}

// refreshPrompts fetches the full prompt list from the server, stores it in
// the cache and invokes onPromptListChanged if set.
func (c *Client) refreshPrompts(ctx context.Context) {
	sess, err := c.getSession()
	if err != nil {
		return
	}
	var prompts []llm.Prompt
	var cursor string
	for {
		result, err := sess.ListPrompts(ctx, &sdkmcp.ListPromptsParams{Cursor: cursor})
		if err != nil {
			return
		}
		for _, p := range result.Prompts {
			prompts = append(prompts, &mcpPrompt{p: p})
		}
		if result.NextCursor == "" {
			break
		}
		cursor = result.NextCursor
	}
	c.mu.Lock()
	c.prompts = prompts
	fn := c.onPromptListChanged
	c.mu.Unlock()
	if fn != nil {
		fn(ctx)
	}
}
