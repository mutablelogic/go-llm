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

// ListAgents returns externally exposed agents matching the given request parameters.
func (c *Client) ListAgents(ctx context.Context, req schema.AgentListRequest) (*schema.AgentList, error) {
	var response schema.AgentList
	if err := c.DoWithContext(ctx, client.MethodGet, &response, client.OptPath("agent"), client.OptQuery(req.Query())); err != nil {
		return nil, err
	}

	return &response, nil
}

// GetAgent returns metadata for a specific agent by name.
func (c *Client) GetAgent(ctx context.Context, name string) (*schema.AgentMeta, error) {
	if name == "" {
		return nil, fmt.Errorf("agent name cannot be empty")
	}

	var response schema.AgentMeta
	if err := c.DoWithContext(ctx, client.MethodGet, &response, client.OptPath("agent", name)); err != nil {
		return nil, err
	}

	return &response, nil
}
