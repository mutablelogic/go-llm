package httpclient

import (
	"context"
	"fmt"

	// Packages
	client "github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
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

// CallAgent executes an agent and returns the raw result as an llm.Resource.
func (c *Client) CallAgent(ctx context.Context, name string, req schema.CallAgentRequest) (llm.Resource, error) {
	if name == "" {
		return nil, fmt.Errorf("agent name cannot be empty")
	}

	payload, err := client.NewJSONRequest(req)
	if err != nil {
		return nil, err
	}

	resource := new(resource)
	err = c.DoWithContext(ctx, payload, resource, client.OptPath("agent", name))
	if err != nil {
		return nil, err
	}
	if resource.empty() {
		return nil, nil
	}

	return resource, nil
}
