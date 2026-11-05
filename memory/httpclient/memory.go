package httpclient

import (
	"context"
	"net/http"
	"net/url"

	// Packages
	client "github.com/mutablelogic/go-client"
	schema "github.com/mutablelogic/go-llm/memory/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// ListMemories returns the memories, the most recently changed first, as the
// request selects, and how many there are
func (c *Client) ListMemories(ctx context.Context, req schema.MemoryListRequest) (*schema.MemoryList, error) {
	var response schema.MemoryList
	if err := c.DoWithContext(ctx, client.NewRequestEx(http.MethodGet, types.ContentTypeJSON), &response, client.OptPath("memory", "memory"), client.OptQuery(req.Query())); err != nil {
		return nil, err
	}
	return &response, nil
}

// GetMemory returns the memory with the key
func (c *Client) GetMemory(ctx context.Context, key string) (*schema.Memory, error) {
	var response schema.Memory
	if err := c.DoWithContext(ctx, client.NewRequestEx(http.MethodGet, types.ContentTypeJSON), &response, client.OptPath("memory", "memory", key)); err != nil {
		return nil, err
	}
	return &response, nil
}

// RememberMemory remembers the memory with the key, which is created, when
// the request has no version, or replaces the memory at the version, and
// returns it
func (c *Client) RememberMemory(ctx context.Context, key string, req schema.MemoryRequest) (*schema.Memory, error) {
	request, err := client.NewJSONRequestEx(http.MethodPut, req, types.ContentTypeJSON)
	if err != nil {
		return nil, err
	}
	var response schema.Memory
	if err := c.DoWithContext(ctx, request, &response, client.OptPath("memory", "memory", key)); err != nil {
		return nil, err
	}
	return &response, nil
}

// ForgetMemory forgets the memory with the key, or, with a line, only the
// line of it, and returns the memory, as it is, or was, when it's forgotten
func (c *Client) ForgetMemory(ctx context.Context, key, line string) (*schema.Memory, error) {
	opts := []client.RequestOpt{client.OptPath("memory", "memory", key)}
	if line != "" {
		opts = append(opts, client.OptQuery(url.Values{"line": {line}}))
	}
	var response schema.Memory
	if err := c.DoWithContext(ctx, client.NewRequestEx(http.MethodDelete, types.ContentTypeJSON), &response, opts...); err != nil {
		return nil, err
	}
	return &response, nil
}
