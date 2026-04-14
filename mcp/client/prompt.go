package client

import (
	"context"
	"encoding/json"

	// Packages
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/kernel/schema"
	opt "github.com/mutablelogic/go-llm/pkg/opt"
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
func (m *mcpPrompt) Prepare(_ context.Context, _ ...llm.Resource) (string, []opt.Opt, error) {
	return "", nil, schema.ErrNotImplemented.With("Prepare not supported for MCP prompts")
}

func (m *mcpPrompt) MarshalJSON() ([]byte, error) {
	if m == nil || m.p == nil {
		return []byte("null"), nil
	}
	return json.Marshal(struct {
		Name        string                   `json:"name"`
		Title       string                   `json:"title,omitempty"`
		Description string                   `json:"description,omitempty"`
		Arguments   []*sdkmcp.PromptArgument `json:"arguments,omitempty"`
	}{
		Name:        m.p.Name,
		Title:       m.p.Title,
		Description: m.p.Description,
		Arguments:   m.p.Arguments,
	})
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

// GetPrompt fetches a prepared prompt from the connected MCP server.
func (c *Client) GetPrompt(ctx context.Context, name string, arguments map[string]string) (*sdkmcp.GetPromptResult, error) {
	sess, err := c.getSession()
	if err != nil {
		return nil, err
	}
	return sess.GetPrompt(ctx, &sdkmcp.GetPromptParams{
		Name:      name,
		Arguments: arguments,
	})
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
