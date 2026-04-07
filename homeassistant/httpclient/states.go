package homeassistant

import (
	"context"
	"encoding/json"

	// Packages
	"github.com/mutablelogic/go-client"
	"github.com/mutablelogic/go-llm/homeassistant/schema"
)

///////////////////////////////////////////////////////////////////////////////
// API CALLS

// States returns all the entities and their state
func (c *Client) States(ctx context.Context) ([]*schema.State, error) {
	// Return the response
	var response []*schema.State
	if err := c.DoWithContext(ctx, nil, &response, client.OptPath("states")); err != nil {
		return nil, err
	}

	// Return success
	return response, nil
}

// State returns a state for a specific entity
func (c *Client) State(ctx context.Context, EntityId string) (*schema.State, error) {
	// Return the response
	var response *schema.State
	if err := c.DoWithContext(ctx, nil, &response, client.OptPath("states", EntityId)); err != nil {
		return response, err
	}

	// Return success
	return response, nil
}

// SetState creates or updates the state of an entity. The state string is required;
// attributes is optional. Returns the resulting state object.
// Note: This sets the representation within Home Assistant — it does not communicate
// with the actual device. Use Call() to actuate a device.
func (c *Client) SetState(ctx context.Context, entityId, state string, attributes map[string]any) (*schema.State, error) {
	req := map[string]any{
		"state": state,
	}
	if len(attributes) > 0 {
		req["attributes"] = attributes
	}

	payload, err := client.NewJSONRequest(req)
	if err != nil {
		return nil, err
	}

	var response schema.State
	if err := c.DoWithContext(ctx, payload, &response, client.OptPath("states", entityId)); err != nil {
		return nil, err
	}

	return &response, nil
}

// DeleteState removes an entity from Home Assistant.
func (c *Client) DeleteState(ctx context.Context, entityId string) error {
	var response json.RawMessage
	if err := c.DoWithContext(ctx, client.MethodDelete, &response, client.OptPath("states", entityId)); err != nil {
		return err
	}
	return nil
}
