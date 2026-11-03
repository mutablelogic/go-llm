package httpclient

////////////////////////////////////////////////////////////////////////////////
// REQUEST TYPES

// GetStatesRequest is the input for listing the entities and their states
type GetStatesRequest struct {
	Domain string `json:"domain,omitempty" help:"Filter entities by domain (e.g. light, switch, sensor, climate). Returns all entities when empty."`
}

// GetStateRequest is the input for getting the state of an entity
type GetStateRequest struct {
	EntityId string `json:"entity_id" help:"The entity ID to query (e.g. light.living_room)."`
}

// GetServicesRequest is the input for listing the services of a domain
type GetServicesRequest struct {
	Domain string `json:"domain" help:"The domain to list services for (e.g. light, switch, climate)."`
}

// CallServiceRequest is the input for calling a service
type CallServiceRequest struct {
	Domain  string         `json:"domain" help:"The service domain (e.g. light, switch, climate, media_player)."`
	Service string         `json:"service" help:"The service to call (e.g. turn_on, turn_off, toggle, set_temperature)."`
	Data    map[string]any `json:"data,omitempty" help:"Service data including entity_id and any service-specific fields."`
}

// SetStateRequest is the input for setting the state of an entity
type SetStateRequest struct {
	EntityId   string         `json:"entity_id" help:"The entity ID to set (e.g. sensor.my_sensor)."`
	State      string         `json:"state" help:"The state value to set."`
	Attributes map[string]any `json:"attributes,omitempty" help:"Optional attributes such as unit_of_measurement, friendly_name, etc."`
}

// FireEventRequest is the input for firing an event
type FireEventRequest struct {
	EventType string         `json:"event_type" help:"The event type to fire (e.g. my_custom_event)."`
	EventData map[string]any `json:"event_data,omitempty" help:"Optional data to include with the event."`
}

// RenderTemplateRequest is the input for rendering a template
type RenderTemplateRequest struct {
	Template string `json:"template" help:"The Jinja2 template string to render (e.g. '{{ states(\"sensor.temperature\") }}')."`
}
