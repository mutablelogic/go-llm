package homeassistant

import (
	"context"
	"encoding/json"
	"slices"
	"strings"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	toolkit "github.com/mutablelogic/go-llm/toolkit"
	httpclient "github.com/mutablelogic/go-llm/toolkit/homeassistant/httpclient"
	jsonschema "github.com/mutablelogic/go-server/pkg/jsonschema"
)

////////////////////////////////////////////////////////////////////////////////
// TYPES

type getStates struct {
	toolkit.Base
	client *httpclient.Client
}

type getState struct {
	toolkit.Base
	client *httpclient.Client
}

type getServices struct {
	toolkit.Base
	client *httpclient.Client
}

type callService struct {
	toolkit.Base
	client *httpclient.Client
}

type setState struct {
	toolkit.Base
	client *httpclient.Client
}

type fireEvent struct {
	toolkit.Base
	client *httpclient.Client
}

type renderTemplate struct {
	toolkit.Base
	client *httpclient.Client
}

var _ llm.Tool = (*getStates)(nil)
var _ llm.Tool = (*getState)(nil)
var _ llm.Tool = (*getServices)(nil)
var _ llm.Tool = (*callService)(nil)
var _ llm.Tool = (*setState)(nil)
var _ llm.Tool = (*fireEvent)(nil)
var _ llm.Tool = (*renderTemplate)(nil)

// stateSummary is a compact summary of an entity's state, for a model
type stateSummary struct {
	EntityId string `json:"entity_id"`
	State    string `json:"state"`
	Name     string `json:"name"`
	Class    string `json:"class,omitempty"`
	Unit     string `json:"unit,omitempty"`
}

// serviceSummary is a compact summary of a service, for a model
type serviceSummary struct {
	Call        string   `json:"call"`
	Name        string   `json:"name,omitempty"`
	Description string   `json:"description,omitempty"`
	Fields      []string `json:"fields,omitempty"`
}

////////////////////////////////////////////////////////////////////////////////
// GET STATES

func (*getStates) Name() string {
	return "get_states"
}

func (*getStates) Description() string {
	return "List all Home Assistant entities and their current state. " +
		"Optionally filter by domain (e.g. light, switch, sensor, climate, media_player). " +
		"Returns entity ID, state value, friendly name, and key attributes."
}

// InputSchema returns the JSON schema for the tool input
func (*getStates) InputSchema() *jsonschema.Schema {
	return jsonschema.MustFor[httpclient.GetStatesRequest]()
}

// Run the tool with the input
func (t *getStates) Run(ctx context.Context, input json.RawMessage) (any, error) {
	var req httpclient.GetStatesRequest
	if err := unmarshal(input, &req); err != nil {
		return nil, err
	}

	states, err := t.client.States(ctx)
	if err != nil {
		return nil, err
	}

	// Return a summary of the available entities, in the domain if set
	domain := strings.TrimSpace(req.Domain)
	result := make([]stateSummary, 0, len(states))
	for _, s := range states {
		if domain != "" && s.Domain() != domain {
			continue
		} else if s.Value() == "" {
			continue
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

////////////////////////////////////////////////////////////////////////////////
// GET STATE

func (*getState) Name() string {
	return "get_state"
}

func (*getState) Description() string {
	return "Get the full state of a specific Home Assistant entity by its entity ID. " +
		"Returns the state value, all attributes, and timestamps."
}

// InputSchema returns the JSON schema for the tool input
func (*getState) InputSchema() *jsonschema.Schema {
	return jsonschema.MustFor[httpclient.GetStateRequest]()
}

// Run the tool with the input
func (t *getState) Run(ctx context.Context, input json.RawMessage) (any, error) {
	var req httpclient.GetStateRequest
	if err := unmarshal(input, &req); err != nil {
		return nil, err
	}
	if req.EntityId = strings.TrimSpace(req.EntityId); req.EntityId == "" {
		return nil, llm.ErrBadParameter.With("entity_id is required")
	}

	return t.client.State(ctx, req.EntityId)
}

////////////////////////////////////////////////////////////////////////////////
// GET SERVICES

func (*getServices) Name() string {
	return "get_services"
}

func (*getServices) Description() string {
	return "List available services for a Home Assistant domain. " +
		"Returns service names and descriptions so you know what actions can be performed. " +
		"Use this before calling call_service if you are unsure which services are available."
}

// InputSchema returns the JSON schema for the tool input
func (*getServices) InputSchema() *jsonschema.Schema {
	return jsonschema.MustFor[httpclient.GetServicesRequest]()
}

// Run the tool with the input
func (t *getServices) Run(ctx context.Context, input json.RawMessage) (any, error) {
	var req httpclient.GetServicesRequest
	if err := unmarshal(input, &req); err != nil {
		return nil, err
	}
	if req.Domain = strings.TrimSpace(req.Domain); req.Domain == "" {
		return nil, llm.ErrBadParameter.With("domain is required")
	}

	services, err := t.client.Services(ctx, req.Domain)
	if err != nil {
		return nil, err
	}

	// Return a summary of the services, with the names of their fields
	result := make([]serviceSummary, 0, len(services))
	for _, s := range services {
		summary := serviceSummary{
			Call:        s.Call,
			Name:        s.Name,
			Description: s.Description,
		}
		for name := range s.Fields {
			summary.Fields = append(summary.Fields, name)
		}
		slices.Sort(summary.Fields)
		result = append(result, summary)
	}
	return result, nil
}

////////////////////////////////////////////////////////////////////////////////
// CALL SERVICE

func (*callService) Name() string {
	return "call_service"
}

func (*callService) Description() string {
	return "Call a Home Assistant service to control a device. " +
		"Common examples: domain='light' service='turn_on' data={'entity_id':'light.living_room','brightness':255}, " +
		"domain='switch' service='toggle' data={'entity_id':'switch.fan'}, " +
		"domain='climate' service='set_temperature' data={'entity_id':'climate.thermostat','temperature':22}. " +
		"Returns the list of states that changed."
}

// InputSchema returns the JSON schema for the tool input
func (*callService) InputSchema() *jsonschema.Schema {
	return jsonschema.MustFor[httpclient.CallServiceRequest]()
}

// Run the tool with the input
func (t *callService) Run(ctx context.Context, input json.RawMessage) (any, error) {
	var req httpclient.CallServiceRequest
	if err := unmarshal(input, &req); err != nil {
		return nil, err
	}
	if req.Domain = strings.TrimSpace(req.Domain); req.Domain == "" {
		return nil, llm.ErrBadParameter.With("domain is required")
	}
	if req.Service = strings.TrimSpace(req.Service); req.Service == "" {
		return nil, llm.ErrBadParameter.With("service is required")
	}

	return t.client.Call(ctx, req.Domain, req.Service, req.Data)
}

////////////////////////////////////////////////////////////////////////////////
// SET STATE

func (*setState) Name() string {
	return "set_state"
}

func (*setState) Description() string {
	return "Set or create the state of a Home Assistant entity. " +
		"This updates the representation in Home Assistant but does NOT communicate with the actual device. " +
		"Use call_service to actuate devices. " +
		"Useful for creating virtual sensors or updating helper entities."
}

// InputSchema returns the JSON schema for the tool input
func (*setState) InputSchema() *jsonschema.Schema {
	return jsonschema.MustFor[httpclient.SetStateRequest]()
}

// Run the tool with the input
func (t *setState) Run(ctx context.Context, input json.RawMessage) (any, error) {
	var req httpclient.SetStateRequest
	if err := unmarshal(input, &req); err != nil {
		return nil, err
	}
	if req.EntityId = strings.TrimSpace(req.EntityId); req.EntityId == "" {
		return nil, llm.ErrBadParameter.With("entity_id is required")
	}
	if req.State == "" {
		return nil, llm.ErrBadParameter.With("state is required")
	}

	return t.client.SetState(ctx, req.EntityId, req.State, req.Attributes)
}

////////////////////////////////////////////////////////////////////////////////
// FIRE EVENT

func (*fireEvent) Name() string {
	return "fire_event"
}

func (*fireEvent) Description() string {
	return "Fire an event on the Home Assistant event bus. " +
		"This can trigger automations that listen for the specified event type."
}

// InputSchema returns the JSON schema for the tool input
func (*fireEvent) InputSchema() *jsonschema.Schema {
	return jsonschema.MustFor[httpclient.FireEventRequest]()
}

// Run the tool with the input
func (t *fireEvent) Run(ctx context.Context, input json.RawMessage) (any, error) {
	var req httpclient.FireEventRequest
	if err := unmarshal(input, &req); err != nil {
		return nil, err
	}
	if req.EventType = strings.TrimSpace(req.EventType); req.EventType == "" {
		return nil, llm.ErrBadParameter.With("event_type is required")
	}

	message, err := t.client.FireEvent(ctx, req.EventType, req.EventData)
	if err != nil {
		return nil, err
	}
	return map[string]string{"message": message}, nil
}

////////////////////////////////////////////////////////////////////////////////
// RENDER TEMPLATE

func (*renderTemplate) Name() string {
	return "template"
}

func (*renderTemplate) Description() string {
	return "Render a Home Assistant Jinja2 template. " +
		"Use this to query complex state expressions, perform calculations, or format data. " +
		"Examples: '{{ states(\"sensor.temperature\") }}', " +
		"'{{ states.light | selectattr(\"state\",\"eq\",\"on\") | list | count }} lights are on', " +
		"'{{ as_timestamp(now()) - as_timestamp(states.sensor.last_motion.last_changed) | int }} seconds since motion'."
}

// InputSchema returns the JSON schema for the tool input
func (*renderTemplate) InputSchema() *jsonschema.Schema {
	return jsonschema.MustFor[httpclient.RenderTemplateRequest]()
}

// Run the tool with the input
func (t *renderTemplate) Run(ctx context.Context, input json.RawMessage) (any, error) {
	var req httpclient.RenderTemplateRequest
	if err := unmarshal(input, &req); err != nil {
		return nil, err
	}
	if strings.TrimSpace(req.Template) == "" {
		return nil, llm.ErrBadParameter.With("template is required")
	}

	result, err := t.client.Template(ctx, req.Template)
	if err != nil {
		return nil, err
	}
	return map[string]string{"result": result}, nil
}

////////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// unmarshal decodes the tool input, which may be empty, into v
func unmarshal(input json.RawMessage, v any) error {
	if len(input) == 0 {
		return nil
	}
	if err := json.Unmarshal(input, v); err != nil {
		return llm.ErrBadParameter.Withf("invalid input: %v", err)
	}
	return nil
}
