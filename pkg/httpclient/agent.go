package httpclient

import (
	"context"
	"fmt"
	"net/http"

	// Packages
	client "github.com/mutablelogic/go-client"
	opt "github.com/mutablelogic/go-llm/pkg/opt"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// ListAgents returns a list of all agents.
// Use WithLimit, WithOffset and WithName to filter and paginate results.
func (c *Client) ListAgents(ctx context.Context, opts ...opt.Opt) (*schema.ListAgentResponse, error) {
	// Apply options
	o, err := opt.Apply(opts...)
	if err != nil {
		return nil, err
	}

	// Create request
	req := client.NewRequest()
	reqOpts := []client.RequestOpt{client.OptPath("agent")}
	if q := o.Query(opt.LimitKey, opt.OffsetKey, opt.NameKey, opt.VersionKey); len(q) > 0 {
		reqOpts = append(reqOpts, client.OptQuery(q))
	}

	// Perform request
	var response schema.ListAgentResponse
	if err := c.DoWithContext(ctx, req, &response, reqOpts...); err != nil {
		return nil, err
	}

	// Return the response
	return &response, nil
}

// GetAgent retrieves an agent by ID or name.
func (c *Client) GetAgent(ctx context.Context, id string) (*schema.Agent, error) {
	if id == "" {
		return nil, fmt.Errorf("agent ID cannot be empty")
	}

	// Create request
	req := client.NewRequest()
	reqOpts := []client.RequestOpt{client.OptPath("agent", id)}

	// Perform request
	var response schema.Agent
	if err := c.DoWithContext(ctx, req, &response, reqOpts...); err != nil {
		return nil, err
	}

	// Return the response
	return &response, nil
}

// CreateAgent creates a new agent with the given metadata (sent as JSON).
func (c *Client) CreateAgent(ctx context.Context, meta schema.AgentMeta) (*schema.Agent, error) {
	// Create request
	req, err := client.NewJSONRequest(meta)
	if err != nil {
		return nil, err
	}
	reqOpts := []client.RequestOpt{client.OptPath("agent")}

	// Perform request
	var response schema.Agent
	if err := c.DoWithContext(ctx, req, &response, reqOpts...); err != nil {
		return nil, err
	}

	// Return the response
	return &response, nil
}

// UpdateAgent updates an existing agent by name (sent as JSON).
// The agent name in the metadata is used to look up the existing agent.
func (c *Client) UpdateAgent(ctx context.Context, meta schema.AgentMeta) (*schema.Agent, error) {
	// Create request
	req, err := client.NewJSONRequestEx(http.MethodPut, meta, "")
	if err != nil {
		return nil, err
	}
	reqOpts := []client.RequestOpt{client.OptPath("agent")}

	// Perform request
	var response schema.Agent
	if err := c.DoWithContext(ctx, req, &response, reqOpts...); err != nil {
		return nil, err
	}

	// Return the response
	return &response, nil
}

// DeleteAgent deletes an agent by ID or name.
func (c *Client) DeleteAgent(ctx context.Context, id string) error {
	if id == "" {
		return fmt.Errorf("agent ID cannot be empty")
	}

	// Create request
	reqOpts := []client.RequestOpt{client.OptPath("agent", id)}

	// Perform request
	if err := c.DoWithContext(ctx, client.MethodDelete, nil, reqOpts...); err != nil {
		return err
	}

	// Return success
	return nil
}
