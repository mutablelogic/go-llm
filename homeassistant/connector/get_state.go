package homeassistant

import (
	"context"
	"encoding/json"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	httpclient "github.com/mutablelogic/go-llm/homeassistant/httpclient"
	"github.com/mutablelogic/go-llm/kernel/schema"
	"github.com/mutablelogic/go-llm/pkg/tool"
	jsonschema "github.com/mutablelogic/go-server/pkg/jsonschema"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// GetStateRequest returns a single entity's state.
type GetStateRequest struct {
	EntityId string `json:"entity_id" jsonschema:"The entity ID to query (e.g. light.living_room)."`
}

type getState struct {
	tool.DefaultTool
	client *httpclient.Client
}

var _ llm.Tool = (*getState)(nil)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

func (*getState) Name() string { return "ha_get_state" }

func (*getState) Description() string {
	return "Get the full state of a specific Home Assistant entity by its entity ID. " +
		"Returns the state value, all attributes, and timestamps."
}

func (*getState) InputSchema() *jsonschema.Schema { return jsonschema.MustFor[GetStateRequest]() }

func (t *getState) Run(ctx context.Context, input json.RawMessage) (any, error) {
	var req GetStateRequest
	if len(input) > 0 {
		if err := json.Unmarshal(input, &req); err != nil {
			return nil, schema.ErrBadParameter.Withf("failed to unmarshal input: %v", err)
		}
	}
	if req.EntityId == "" {
		return nil, schema.ErrBadParameter.With("entity_id is required")
	}

	return t.client.State(ctx, req.EntityId)
}
