package client

import (
	"context"
	"encoding/json"

	// Packages
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	llm "github.com/mutablelogic/go-llm"
	opt "github.com/mutablelogic/go-llm/pkg/opt"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
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

// Prepare is not supported for remote MCP prompts — execution is always
// delegated back through the toolkit's delegate.
func (m *mcpPrompt) Prepare(_ context.Context, _ json.RawMessage) (string, []opt.Opt, error) {
	return "", nil, schema.ErrNotImplemented.With("Prepare not supported for MCP prompts")
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// ListPrompts returns the cached list of prompts advertised by the connected
// MCP server. The cache is populated on connect and refreshed automatically
// on each PromptListChanged notification. Returns ErrServiceUnavailable if not active.
func (c *Client) ListPrompts(_ context.Context) ([]llm.Prompt, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.session == nil {
		return nil, schema.ErrServiceUnavailable
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
	c.mu.Lock()
	prompts := make([]llm.Prompt, 0, len(c.prompts)+10)
	c.mu.Unlock()
	for cursor := ""; ; {
		result, err := sess.ListPrompts(ctx, &sdkmcp.ListPromptsParams{Cursor: cursor})
		if err != nil {
			return
		}
		for _, p := range result.Prompts {
			prompts = append(prompts, &mcpPrompt{p: p})
		}
		if cursor = result.NextCursor; cursor == "" {
			break
		}
	}
	c.mu.Lock()
	c.prompts = prompts
	fn := c.onPromptListChanged
	c.mu.Unlock()
	if fn != nil {
		fn(ctx)
	}
}
