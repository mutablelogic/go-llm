package homeassistant

import (
	"context"
	"encoding/json"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	httpclient "github.com/mutablelogic/go-llm/homeassistant/httpclient"
	hasschema "github.com/mutablelogic/go-llm/homeassistant/schema"
	schema "github.com/mutablelogic/go-llm/kernel/schema"
	tool "github.com/mutablelogic/go-llm/toolkit/tool"
	jsonschema "github.com/mutablelogic/go-server/pkg/jsonschema"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// GetStatesRequest filters the list of entity states.
type GetStatesRequest struct {
	Domain string `json:"domain,omitempty" jsonschema:"Filter entities by domain (e.g. light, switch, sensor, climate). Returns all entities when empty."`
}

type getStates struct {
	tool.Base
	client *httpclient.Client
}

var _ llm.Tool = (*getStates)(nil)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

func (*getStates) Name() string {
	return "ha_get_states"
}

func (*getStates) Description() string {
	return "List all Home Assistant entities and their current state. " +
		"Optionally filter by domain (e.g. light, switch, sensor, climate, media_player). " +
		"Returns entity ID, state value, friendly name, and key attributes."
}

func (*getStates) InputSchema() *jsonschema.Schema { return jsonschema.MustFor[GetStatesRequest]() }

func (t *getStates) Run(ctx context.Context, input json.RawMessage) (any, error) {
	var req GetStatesRequest
	if len(input) > 0 {
		if err := json.Unmarshal(input, &req); err != nil {
			return nil, schema.ErrBadParameter.Withf("failed to unmarshal input: %v", err)
		}
	}

	states, err := t.client.States(ctx)
	if err != nil {
		return nil, err
	}

	// Filter by domain if requested
	if req.Domain != "" {
		filtered := make([]*hasschema.State, 0, len(states))
		for _, s := range states {
			if s.Domain() == req.Domain {
				filtered = append(filtered, s)
			}
		}
		states = filtered
	}

	// Return a compact summary for the LLM
	type stateSummary struct {
		EntityId string `json:"entity_id"`
		State    string `json:"state"`
		Name     string `json:"name"`
		Class    string `json:"class,omitempty"`
		Unit     string `json:"unit,omitempty"`
	}
	result := make([]stateSummary, 0, len(states))
	for _, s := range states {
		if s.Value() == "" {
			continue // skip unavailable entities
		}
		result = append(result, stateSummary{
			EntityId: s.Entity,
			State:    s.Value(),
			Name:     s.Name(),
			Class:    s.Class(),
			Unit:     s.UnitOfMeasurement(),
		})
	}
	return result, nil
}
