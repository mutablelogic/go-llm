package homeassistant

import (
	"context"
	"encoding/json"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	httpclient "github.com/mutablelogic/go-llm/homeassistant/httpclient"
	schema "github.com/mutablelogic/go-llm/kernel/schema"
	tool "github.com/mutablelogic/go-llm/toolkit/tool"
	jsonschema "github.com/mutablelogic/go-server/pkg/jsonschema"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// SetStateRequest creates or updates an entity's state representation.
type SetStateRequest struct {
	EntityId   string         `json:"entity_id" jsonschema:"The entity ID to set (e.g. sensor.my_sensor)."`
	State      string         `json:"state" jsonschema:"The state value to set."`
	Attributes map[string]any `json:"attributes,omitempty" jsonschema:"Optional attributes such as unit_of_measurement, friendly_name, etc."`
}

type setState struct {
	tool.Base
	client *httpclient.Client
}

var _ llm.Tool = (*setState)(nil)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

func (*setState) Name() string { return "ha_set_state" }

func (*setState) Description() string {
	return "Set or create the state of a Home Assistant entity. " +
		"This updates the representation in Home Assistant but does NOT communicate with the actual device. " +
		"Use ha_call_service to actuate devices. " +
		"Useful for creating virtual sensors or updating helper entities."
}

func (*setState) InputSchema() *jsonschema.Schema { return jsonschema.MustFor[SetStateRequest]() }

func (t *setState) Run(ctx context.Context, input json.RawMessage) (any, error) {
	var req SetStateRequest
	if len(input) > 0 {
		if err := json.Unmarshal(input, &req); err != nil {
			return nil, schema.ErrBadParameter.Withf("failed to unmarshal input: %v", err)
		}
	}
	if req.EntityId == "" {
		return nil, schema.ErrBadParameter.With("entity_id is required")
	}
	if req.State == "" {
		return nil, schema.ErrBadParameter.With("state is required")
	}

	return t.client.SetState(ctx, req.EntityId, req.State, req.Attributes)
}
