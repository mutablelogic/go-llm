package homeassistant

import (
	"context"
	"encoding/json"
	"strings"

	// Packages
	jsonschema "github.com/google/jsonschema-go/jsonschema"
	"github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
	"github.com/mutablelogic/go-llm/pkg/tool"
)

///////////////////////////////////////////////////////////////////////////////
// TOOL TYPES

type getStates struct{ client *Client }
type getState struct{ client *Client }
type callService struct{ client *Client }
type getServices struct{ client *Client }
type setState struct{ client *Client }
type fireEvent struct{ client *Client }
type renderTemplate struct{ client *Client }

var _ tool.Tool = (*getStates)(nil)
var _ tool.Tool = (*getState)(nil)
var _ tool.Tool = (*callService)(nil)
var _ tool.Tool = (*getServices)(nil)
var _ tool.Tool = (*setState)(nil)
var _ tool.Tool = (*fireEvent)(nil)
var _ tool.Tool = (*renderTemplate)(nil)

///////////////////////////////////////////////////////////////////////////////
// REQUEST TYPES

// GetStatesRequest filters the list of entity states.
type GetStatesRequest struct {
	Domain string `json:"domain,omitempty" jsonschema:"Filter entities by domain (e.g. light, switch, sensor, climate). Returns all entities when empty."`
}

// GetStateRequest returns a single entity's state.
type GetStateRequest struct {
	EntityId string `json:"entity_id" jsonschema:"The entity ID to query (e.g. light.living_room)."`
}

// CallServiceRequest calls a Home Assistant service.
type CallServiceRequest struct {
	Domain  string         `json:"domain" jsonschema:"The service domain (e.g. light, switch, climate, media_player)."`
	Service string         `json:"service" jsonschema:"The service to call (e.g. turn_on, turn_off, toggle, set_temperature)."`
	Data    map[string]any `json:"data,omitempty" jsonschema:"Service data including entity_id and any service-specific fields."`
}

// GetServicesRequest lists available services for a domain.
type GetServicesRequest struct {
	Domain string `json:"domain" jsonschema:"The domain to list services for (e.g. light, switch, climate)."`
}

// SetStateRequest creates or updates an entity's state representation.
type SetStateRequest struct {
	EntityId   string         `json:"entity_id" jsonschema:"The entity ID to set (e.g. sensor.my_sensor)."`
	State      string         `json:"state" jsonschema:"The state value to set."`
	Attributes map[string]any `json:"attributes,omitempty" jsonschema:"Optional attributes such as unit_of_measurement, friendly_name, etc."`
}

// FireEventRequest fires a custom event on the event bus.
type FireEventRequest struct {
	EventType string         `json:"event_type" jsonschema:"The event type to fire (e.g. my_custom_event)."`
	EventData map[string]any `json:"event_data,omitempty" jsonschema:"Optional data to include with the event."`
}

// RenderTemplateRequest renders a Jinja2 template.
type RenderTemplateRequest struct {
	Template string `json:"template" jsonschema:"The Jinja2 template string to render (e.g. '{{ states(\"sensor.temperature\") }}')."`
}

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// NewTools returns Home Assistant tools for use with LLM agents.
func NewTools(endPoint, apiKey string, opts ...client.ClientOpt) ([]tool.Tool, error) {
	c, err := New(endPoint, apiKey, opts...)
	if err != nil {
		return nil, err
	}

	return []tool.Tool{
		&getStates{client: c},
		&getState{client: c},
		&callService{client: c},
		&getServices{client: c},
		&setState{client: c},
		&fireEvent{client: c},
		&renderTemplate{client: c},
	}, nil
}

///////////////////////////////////////////////////////////////////////////////
// ha_get_states

func (*getStates) Name() string { return "ha_get_states" }

func (*getStates) Description() string {
	return "List all Home Assistant entities and their current state. " +
		"Optionally filter by domain (e.g. light, switch, sensor, climate, media_player). " +
		"Returns entity ID, state value, friendly name, and key attributes."
}

func (*getStates) Schema() (*jsonschema.Schema, error) {
	return jsonschema.For[GetStatesRequest](nil)
}

func (t *getStates) Run(ctx context.Context, input json.RawMessage) (any, error) {
	var req GetStatesRequest
	if len(input) > 0 {
		if err := json.Unmarshal(input, &req); err != nil {
			return nil, llm.ErrBadParameter.Withf("failed to unmarshal input: %v", err)
		}
	}

	states, err := t.client.States(ctx)
	if err != nil {
		return nil, err
	}

	// Filter by domain if requested
	if req.Domain != "" {
		filtered := make([]*State, 0, len(states))
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

///////////////////////////////////////////////////////////////////////////////
// ha_get_state

func (*getState) Name() string { return "ha_get_state" }

func (*getState) Description() string {
	return "Get the full state of a specific Home Assistant entity by its entity ID. " +
		"Returns the state value, all attributes, and timestamps."
}

func (*getState) Schema() (*jsonschema.Schema, error) {
	return jsonschema.For[GetStateRequest](nil)
}

func (t *getState) Run(ctx context.Context, input json.RawMessage) (any, error) {
	var req GetStateRequest
	if len(input) > 0 {
		if err := json.Unmarshal(input, &req); err != nil {
			return nil, llm.ErrBadParameter.Withf("failed to unmarshal input: %v", err)
		}
	}
	if req.EntityId == "" {
		return nil, llm.ErrBadParameter.With("entity_id is required")
	}

	return t.client.State(ctx, req.EntityId)
}

///////////////////////////////////////////////////////////////////////////////
// ha_call_service

func (*callService) Name() string { return "ha_call_service" }

func (*callService) Description() string {
	return "Call a Home Assistant service to control a device. " +
		"Common examples: domain='light' service='turn_on' data={'entity_id':'light.living_room','brightness':255}, " +
		"domain='switch' service='toggle' data={'entity_id':'switch.fan'}, " +
		"domain='climate' service='set_temperature' data={'entity_id':'climate.thermostat','temperature':22}. " +
		"Returns the list of states that changed."
}

func (*callService) Schema() (*jsonschema.Schema, error) {
	return jsonschema.For[CallServiceRequest](nil)
}

func (t *callService) Run(ctx context.Context, input json.RawMessage) (any, error) {
	var req CallServiceRequest
	if len(input) > 0 {
		if err := json.Unmarshal(input, &req); err != nil {
			return nil, llm.ErrBadParameter.Withf("failed to unmarshal input: %v", err)
		}
	}
	if req.Domain == "" {
		return nil, llm.ErrBadParameter.With("domain is required")
	}
	if req.Service == "" {
		return nil, llm.ErrBadParameter.With("service is required")
	}

	// If entity_id is provided at the top level of data, extract the domain
	// to help the user avoid having to specify it separately
	if req.Data == nil {
		req.Data = map[string]any{}
	}

	return t.client.Call(ctx, req.Domain, req.Service, req.Data)
}

///////////////////////////////////////////////////////////////////////////////
// ha_get_services

func (*getServices) Name() string { return "ha_get_services" }

func (*getServices) Description() string {
	return "List available services for a Home Assistant domain. " +
		"Returns service names and descriptions so you know what actions can be performed. " +
		"Use this before calling ha_call_service if you are unsure which services are available."
}

func (*getServices) Schema() (*jsonschema.Schema, error) {
	return jsonschema.For[GetServicesRequest](nil)
}

func (t *getServices) Run(ctx context.Context, input json.RawMessage) (any, error) {
	var req GetServicesRequest
	if len(input) > 0 {
		if err := json.Unmarshal(input, &req); err != nil {
			return nil, llm.ErrBadParameter.Withf("failed to unmarshal input: %v", err)
		}
	}
	if req.Domain == "" {
		return nil, llm.ErrBadParameter.With("domain is required")
	}

	services, err := t.client.Services(ctx, req.Domain)
	if err != nil {
		return nil, err
	}

	// Return a compact view with name, call ID, and description
	type serviceSummary struct {
		Call        string   `json:"call"`
		Name        string   `json:"name,omitempty"`
		Description string   `json:"description,omitempty"`
		Fields      []string `json:"fields,omitempty"`
	}
	result := make([]serviceSummary, 0, len(services))
	for _, s := range services {
		summary := serviceSummary{
			Call:        s.Call,
			Name:        s.Name,
			Description: s.Description,
		}
		for fieldName := range s.Fields {
			summary.Fields = append(summary.Fields, fieldName)
		}
		result = append(result, summary)
	}
	return result, nil
}

///////////////////////////////////////////////////////////////////////////////
// ha_set_state

func (*setState) Name() string { return "ha_set_state" }

func (*setState) Description() string {
	return "Set or create the state of a Home Assistant entity. " +
		"This updates the representation in Home Assistant but does NOT communicate with the actual device. " +
		"Use ha_call_service to actuate devices. " +
		"Useful for creating virtual sensors or updating helper entities."
}

func (*setState) Schema() (*jsonschema.Schema, error) {
	return jsonschema.For[SetStateRequest](nil)
}

func (t *setState) Run(ctx context.Context, input json.RawMessage) (any, error) {
	var req SetStateRequest
	if len(input) > 0 {
		if err := json.Unmarshal(input, &req); err != nil {
			return nil, llm.ErrBadParameter.Withf("failed to unmarshal input: %v", err)
		}
	}
	if req.EntityId == "" {
		return nil, llm.ErrBadParameter.With("entity_id is required")
	}
	if req.State == "" {
		return nil, llm.ErrBadParameter.With("state is required")
	}

	return t.client.SetState(ctx, req.EntityId, req.State, req.Attributes)
}

///////////////////////////////////////////////////////////////////////////////
// ha_fire_event

func (*fireEvent) Name() string { return "ha_fire_event" }

func (*fireEvent) Description() string {
	return "Fire an event on the Home Assistant event bus. " +
		"This can trigger automations that listen for the specified event type."
}

func (*fireEvent) Schema() (*jsonschema.Schema, error) {
	return jsonschema.For[FireEventRequest](nil)
}

func (t *fireEvent) Run(ctx context.Context, input json.RawMessage) (any, error) {
	var req FireEventRequest
	if len(input) > 0 {
		if err := json.Unmarshal(input, &req); err != nil {
			return nil, llm.ErrBadParameter.Withf("failed to unmarshal input: %v", err)
		}
	}
	if req.EventType == "" {
		return nil, llm.ErrBadParameter.With("event_type is required")
	}

	msg, err := t.client.FireEvent(ctx, req.EventType, req.EventData)
	if err != nil {
		return nil, err
	}
	return map[string]string{"message": msg}, nil
}

///////////////////////////////////////////////////////////////////////////////
// ha_template

func (*renderTemplate) Name() string { return "ha_template" }

func (*renderTemplate) Description() string {
	return "Render a Home Assistant Jinja2 template. " +
		"Use this to query complex state expressions, perform calculations, or format data. " +
		"Examples: '{{ states(\"sensor.temperature\") }}', " +
		"'{{ states.light | selectattr(\"state\",\"eq\",\"on\") | list | count }} lights are on', " +
		"'{{ as_timestamp(now()) - as_timestamp(states.sensor.last_motion.last_changed) | int }} seconds since motion'."
}

func (*renderTemplate) Schema() (*jsonschema.Schema, error) {
	return jsonschema.For[RenderTemplateRequest](nil)
}

func (t *renderTemplate) Run(ctx context.Context, input json.RawMessage) (any, error) {
	var req RenderTemplateRequest
	if len(input) > 0 {
		if err := json.Unmarshal(input, &req); err != nil {
			return nil, llm.ErrBadParameter.Withf("failed to unmarshal input: %v", err)
		}
	}
	if req.Template == "" {
		return nil, llm.ErrBadParameter.With("template is required")
	}

	result, err := t.client.Template(ctx, req.Template)
	if err != nil {
		return nil, err
	}
	return map[string]string{"result": strings.TrimSpace(result)}, nil
}
