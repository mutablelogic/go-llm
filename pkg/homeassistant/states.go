package homeassistant

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	// Packages
	"github.com/mutablelogic/go-client"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type State struct {
	Entity       string         `json:"entity_id,width:40"`
	LastChanged  time.Time      `json:"last_changed,omitempty"`
	LastReported time.Time      `json:"last_reported,omitempty"`
	LastUpdated  time.Time      `json:"last_updated,omitempty"`
	State        string         `json:"state"`
	Attributes   map[string]any `json:"attributes"`
	Context      struct {
		Id       string `json:"id,omitempty"`
		ParentId string `json:"parent_id,omitempty"`
		UserId   string `json:"user_id,omitempty"`
	} `json:"context"`
}

///////////////////////////////////////////////////////////////////////////////
// API CALLS

// States returns all the entities and their state
func (c *Client) States(ctx context.Context) ([]*State, error) {
	// Return the response
	var response []*State
	if err := c.DoWithContext(ctx, nil, &response, client.OptPath("states")); err != nil {
		return nil, err
	}

	// Return success
	return response, nil
}

// State returns a state for a specific entity
func (c *Client) State(ctx context.Context, EntityId string) (*State, error) {
	// Return the response
	var response *State
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
func (c *Client) SetState(ctx context.Context, entityId, state string, attributes map[string]any) (*State, error) {
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

	var response State
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

///////////////////////////////////////////////////////////////////////////////
// STRINGIFY

func (s State) String() string {
	data, _ := json.MarshalIndent(s, "", "  ")
	return string(data)
}

///////////////////////////////////////////////////////////////////////////////
// METHODS

// Domain is used to determine the services which can be called on the entity
func (s State) Domain() string {
	parts := strings.SplitN(s.Entity, ".", 2)
	if len(parts) == 2 {
		return parts[0]
	} else {
		return ""
	}
}

// Name is the friendly name of the entity
func (s State) Name() string {
	name, ok := s.Attributes["friendly_name"]
	if !ok {
		return s.Entity
	} else if name_, ok := name.(string); !ok {
		return s.Entity
	} else {
		return name_
	}
}

// Value is the current state of the entity, or empty if the state is unavailable
func (s State) Value() string {
	switch strings.ToLower(s.State) {
	case "unavailable", "unknown", "--":
		return ""
	default:
		return s.State
	}
}

// Class determines how the state should be interpreted, or will return "" if it's
// unknown
func (s State) Class() string {
	class, ok := s.Attributes["device_class"]
	if !ok {
		return s.Domain()
	} else if class_, ok := class.(string); !ok {
		return ""
	} else {
		return class_
	}
}

// UnitOfMeasurement provides the unit of measurement for the state, or "" if there
// is no unit of measurement
func (s State) UnitOfMeasurement() string {
	unit, ok := s.Attributes["unit_of_measurement"]
	if !ok {
		return ""
	} else if unit_, ok := unit.(string); !ok {
		return ""
	} else {
		return unit_
	}
}
