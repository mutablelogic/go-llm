package httpclient

import (
	"context"
	"fmt"

	// Packages
	uuid "github.com/google/uuid"
	client "github.com/mutablelogic/go-client"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// ListMessages returns a paginated list of messages for the given session.
func (c *Client) ListMessages(ctx context.Context, session uuid.UUID, req schema.MessageListRequest) (*schema.MessageList, error) {
	if session == uuid.Nil {
		return nil, fmt.Errorf("session ID cannot be nil")
	}

	var response schema.MessageList
	if err := c.DoWithContext(ctx, client.MethodGet, &response,
		client.OptPath("session", session.String(), "message"),
		client.OptQuery(req.Query()),
	); err != nil {
		return nil, err
	}

	return &response, nil
}
