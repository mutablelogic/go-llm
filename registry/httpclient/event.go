package httpclient

import (
	"context"
	"uuid"

	// Packages
	client "github.com/mutablelogic/go-client"
	schema "github.com/mutablelogic/go-llm/registry/schema"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// ListEvents returns the outcomes of events, the most recent first, as the
// request selects, and how many there are
func (c *Client) ListEvents(ctx context.Context, req schema.EventListRequest) (*schema.EventList, error) {
	var response schema.EventList
	if err := c.DoWithContext(ctx, client.MethodGet, &response, client.OptPath("llm", "event"), client.OptQuery(req.Query())); err != nil {
		return nil, err
	}
	return &response, nil
}

// GetEvent returns the outcome of the event with the identifier
func (c *Client) GetEvent(ctx context.Context, id uuid.UUID) (*schema.Event, error) {
	var response schema.Event
	if err := c.DoWithContext(ctx, client.MethodGet, &response, client.OptPath("llm", "event", id.String())); err != nil {
		return nil, err
	}
	return &response, nil
}

// DeleteEvent deletes the outcome of the event with the identifier, and
// returns it, as it was
func (c *Client) DeleteEvent(ctx context.Context, id uuid.UUID) (*schema.Event, error) {
	var response schema.Event
	if err := c.DoWithContext(ctx, client.MethodDelete, &response, client.OptPath("llm", "event", id.String())); err != nil {
		return nil, err
	}
	return &response, nil
}
