package homeassistant

import (
	// Packages
	"context"

	"github.com/mutablelogic/go-client"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type Event struct {
	Event     string `json:"event"`
	Listeners uint   `json:"listener_count"`
}

///////////////////////////////////////////////////////////////////////////////
// API CALLS

// Events returns all the events and number of listeners
func (c *Client) Events(ctx context.Context) ([]Event, error) {
	var response []Event
	if err := c.DoWithContext(ctx, nil, &response, client.OptPath("events")); err != nil {
		return nil, err
	}

	// Return success
	return response, nil
}

// FireEvent fires an event with the given type. Optional event data can be
// passed as a map. Returns a confirmation message.
func (c *Client) FireEvent(ctx context.Context, eventType string, eventData map[string]any) (string, error) {
	type responseMessage struct {
		Message string `json:"message"`
	}

	var payload client.Payload
	var err error
	if len(eventData) > 0 {
		payload, err = client.NewJSONRequest(eventData)
		if err != nil {
			return "", err
		}
	} else {
		payload, err = client.NewJSONRequest(struct{}{})
		if err != nil {
			return "", err
		}
	}

	var response responseMessage
	if err := c.DoWithContext(ctx, payload, &response, client.OptPath("events", eventType)); err != nil {
		return "", err
	}

	return response.Message, nil
}
