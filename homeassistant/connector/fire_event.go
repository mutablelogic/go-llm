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

// FireEventRequest fires a custom event on the event bus.
type FireEventRequest struct {
	EventType string         `json:"event_type" jsonschema:"The event type to fire (e.g. my_custom_event)."`
	EventData map[string]any `json:"event_data,omitempty" jsonschema:"Optional data to include with the event."`
}

type fireEvent struct {
	tool.DefaultTool
	client *httpclient.Client
}

var _ llm.Tool = (*fireEvent)(nil)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

func (*fireEvent) Name() string { return "ha_fire_event" }

func (*fireEvent) Description() string {
	return "Fire an event on the Home Assistant event bus. " +
		"This can trigger automations that listen for the specified event type."
}

func (*fireEvent) InputSchema() *jsonschema.Schema { return jsonschema.MustFor[FireEventRequest]() }

func (t *fireEvent) Run(ctx context.Context, input json.RawMessage) (any, error) {
	var req FireEventRequest
	if len(input) > 0 {
		if err := json.Unmarshal(input, &req); err != nil {
			return nil, schema.ErrBadParameter.Withf("failed to unmarshal input: %v", err)
		}
	}
	if req.EventType == "" {
		return nil, schema.ErrBadParameter.With("event_type is required")
	}

	msg, err := t.client.FireEvent(ctx, req.EventType, req.EventData)
	if err != nil {
		return nil, err
	}

	return map[string]string{"message": msg}, nil
}
