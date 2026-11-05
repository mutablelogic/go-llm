package homeassistant

import (
	"context"
	"encoding/json"
	"slices"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	toolkit "github.com/mutablelogic/go-llm/toolkit"
	httpclient "github.com/mutablelogic/go-llm/toolkit/homeassistant/httpclient"
	jsonschema "github.com/mutablelogic/go-server/pkg/jsonschema"
)

////////////////////////////////////////////////////////////////////////////////
// TYPES

// query is the tool which reads from Home Assistant, as its operation says,
// and changes nothing, so it can be given on its own
type query struct {
	toolkit.Base
	operations map[string]llm.Tool
}

// control is the tool which changes Home Assistant, as its operation says
type control struct {
	toolkit.Base
	operations map[string]llm.Tool
}

var _ llm.Tool = (*query)(nil)
var _ llm.Tool = (*control)(nil)

// queryInput is the input of the query tool: its operation, and what the
// operation needs
type queryInput struct {
	Operation string `json:"operation" enum:"states,state,services,template" help:"states lists the entities, and their states, of a domain, or every domain, state gets all of an entity's state, services lists what can be done with a domain's entities, and template renders a template"`
	Domain    string `json:"domain,omitempty" help:"Domain, such as light, switch, sensor, climate or media_player, whose entities to list, or whose services, which services needs" optional:""`
	EntityId  string `json:"entity_id,omitempty" help:"Entity, such as light.living_room, whose state to get" optional:""`
	Template  string `json:"template,omitempty" help:"Jinja2 template to render, such as '{{ states(\"sensor.temperature\") }}'" optional:""`
}

// controlInput is the input of the control tool: its operation, and what
// the operation needs
type controlInput struct {
	Operation  string         `json:"operation" enum:"call_service,set_state,fire_event" help:"call_service controls devices, set_state sets the state Home Assistant shows an entity has, without changing the device, such as for a virtual sensor, and fire_event fires an event, which automations can be triggered by"`
	Domain     string         `json:"domain,omitempty" help:"Domain of the service, such as light, switch, climate or media_player, to call a service" optional:""`
	Service    string         `json:"service,omitempty" help:"Service, such as turn_on, turn_off, toggle or set_temperature, to call" optional:""`
	Data       map[string]any `json:"data,omitempty" help:"Data of the service, such as its entity_id, and its fields, to call a service" optional:""`
	EntityId   string         `json:"entity_id,omitempty" help:"Entity, such as sensor.my_sensor, whose state to set" optional:""`
	State      string         `json:"state,omitempty" help:"State to set" optional:""`
	Attributes map[string]any `json:"attributes,omitempty" help:"Attributes of the state to set, such as unit_of_measurement, or friendly_name" optional:""`
	EventType  string         `json:"event_type,omitempty" help:"Type of the event to fire, such as my_custom_event" optional:""`
	EventData  map[string]any `json:"event_data,omitempty" help:"Data of the event to fire" optional:""`
}

////////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// newQuery returns the tool which reads from Home Assistant, with the client
func newQuery(client *httpclient.Client) *query {
	return &query{operations: map[string]llm.Tool{
		"states":   &getStates{client: client},
		"state":    &getState{client: client},
		"services": &getServices{client: client},
		"template": &renderTemplate{client: client},
	}}
}

// newControl returns the tool which changes Home Assistant, with the client
func newControl(client *httpclient.Client) *control {
	return &control{operations: map[string]llm.Tool{
		"call_service": &callService{client: client},
		"set_state":    &setState{client: client},
		"fire_event":   &fireEvent{client: client},
	}}
}

////////////////////////////////////////////////////////////////////////////////
// QUERY

func (*query) Name() string {
	return "query"
}

func (*query) Description() string {
	return "Read from Home Assistant, without changing anything: list the entities, and their states, of a domain, or every domain, " +
		"get all of an entity's state, list the services of a domain, to know what can be done with its entities, " +
		"or render a Jinja2 template, to work something out, such as " +
		"'{{ states.light | selectattr(\"state\",\"eq\",\"on\") | list | count }} lights are on'."
}

func (*query) InputSchema() *jsonschema.Schema {
	return jsonschema.MustFor[queryInput]()
}

// Run does the operation
func (t *query) Run(ctx context.Context, input json.RawMessage) (any, error) {
	return run(ctx, t.operations, input)
}

////////////////////////////////////////////////////////////////////////////////
// CONTROL

func (*control) Name() string {
	return "control"
}

func (*control) Description() string {
	return "Change Home Assistant: call a service, to control devices, such as " +
		"domain='light' service='turn_on' data={'entity_id':'light.living_room','brightness':255}, or " +
		"domain='climate' service='set_temperature' data={'entity_id':'climate.thermostat','temperature':22}, " +
		"which returns the states of the entities it targeted, and which changed, as they are now; " +
		"set the state Home Assistant shows an entity has, without changing the device, such as for a virtual sensor; " +
		"or fire an event, which automations can be triggered by."
}

func (*control) InputSchema() *jsonschema.Schema {
	return jsonschema.MustFor[controlInput]()
}

// Run does the operation
func (t *control) Run(ctx context.Context, input json.RawMessage) (any, error) {
	return run(ctx, t.operations, input)
}

////////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// run runs the operation of the input, with the input, which has what the
// operation needs, or returns ErrBadParameter when it isn't one of the
// operations
func run(ctx context.Context, operations map[string]llm.Tool, input json.RawMessage) (any, error) {
	var req struct {
		Operation string `json:"operation"`
	}
	if err := unmarshal(input, &req); err != nil {
		return nil, err
	}
	operation, ok := operations[req.Operation]
	if !ok {
		names := make([]string, 0, len(operations))
		for name := range operations {
			names = append(names, name)
		}
		slices.Sort(names)
		return nil, llm.ErrBadParameter.Withf("unknown operation %q, which isn't one of %s", req.Operation, list(names))
	}
	return operation.Run(ctx, input)
}
