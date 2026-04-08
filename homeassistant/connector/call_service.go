package homeassistant

import (
	"context"
	"encoding/json"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	httpclient "github.com/mutablelogic/go-llm/homeassistant/httpclient"
	homeassistant "github.com/mutablelogic/go-llm/homeassistant/schema"
	schema "github.com/mutablelogic/go-llm/kernel/schema"
	tool "github.com/mutablelogic/go-llm/toolkit/tool"
	jsonschema "github.com/mutablelogic/go-server/pkg/jsonschema"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type CallServiceRequest struct {
	Domain  string         `json:"domain" jsonschema:"The service domain (e.g. light, switch, climate, media_player)."`
	Service string         `json:"service" jsonschema:"The service to call (e.g. turn_on, turn_off, toggle, set_temperature)."`
	Data    map[string]any `json:"data,omitempty" jsonschema:"Service data including entity_id and any service-specific fields."`
}

type CallServiceResponse struct {
	ChangedStates []homeassistant.State `json:"changed_states" jsonschema:"List of states that changed as a result of the service call."`
}

type callService struct {
	tool.Base
	client *httpclient.Client
}

var _ llm.Tool = (*callService)(nil)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

func (*callService) Name() string { return "ha_call_service" }

func (*callService) Description() string {
	return "Call a Home Assistant service to control a device. " +
		"Common examples: domain='light' service='turn_on' data={'entity_id':'light.living_room','brightness':255}, " +
		"domain='switch' service='toggle' data={'entity_id':'switch.fan'}, " +
		"domain='climate' service='set_temperature' data={'entity_id':'climate.thermostat','temperature':22}. " +
		"Returns the list of states that changed."
}

func (*callService) InputSchema() *jsonschema.Schema {
	return jsonschema.MustFor[CallServiceRequest]()
}

func (*callService) OutputSchema() *jsonschema.Schema {
	return jsonschema.MustFor[CallServiceResponse]()
}

func (t *callService) Run(ctx context.Context, input json.RawMessage) (any, error) {
	var req CallServiceRequest
	if len(input) > 0 {
		if err := json.Unmarshal(input, &req); err != nil {
			return nil, schema.ErrBadParameter.Withf("failed to unmarshal input: %v", err)
		}
	}
	if req.Domain == "" {
		return nil, schema.ErrBadParameter.With("domain is required")
	}
	if req.Service == "" {
		return nil, schema.ErrBadParameter.With("service is required")
	}
	if req.Data == nil {
		req.Data = map[string]any{}
	}

	return t.client.Call(ctx, req.Domain, req.Service, req.Data)
}
