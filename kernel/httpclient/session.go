package httpclient

import (
	"context"
	"fmt"
	"net/http"

	// Packages
	uuid "github.com/google/uuid"
	client "github.com/mutablelogic/go-client"
	schema "github.com/mutablelogic/go-llm/kernel/schema"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// ListSessions returns a paginated list of sessions matching the request.
func (c *Client) ListSessions(ctx context.Context, req schema.SessionListRequest) (*schema.SessionList, error) {
	var response schema.SessionList
	if err := c.DoWithContext(ctx, client.MethodGet, &response, client.OptPath("session"), client.OptQuery(req.Query())); err != nil {
		return nil, err
	}

	return &response, nil
}

// CreateSession creates a new session with the given insert data.
func (c *Client) CreateSession(ctx context.Context, req schema.SessionInsert) (*schema.Session, error) {
	httpReq, err := client.NewJSONRequest(req)
	if err != nil {
		return nil, err
	}

	var response schema.Session
	if err := c.DoWithContext(ctx, httpReq, &response, client.OptPath("session")); err != nil {
		return nil, err
	}

	return &response, nil
}

// GetSession returns a session by ID.
func (c *Client) GetSession(ctx context.Context, id uuid.UUID) (*schema.Session, error) {
	if id == uuid.Nil {
		return nil, fmt.Errorf("session ID cannot be nil")
	}

	var response schema.Session
	if err := c.DoWithContext(ctx, client.MethodGet, &response, client.OptPath("session", id.String())); err != nil {
		return nil, err
	}

	return &response, nil
}

// DeleteSession deletes a session by ID and returns the deleted session.
func (c *Client) DeleteSession(ctx context.Context, id uuid.UUID) (*schema.Session, error) {
	if id == uuid.Nil {
		return nil, fmt.Errorf("session ID cannot be nil")
	}

	var response schema.Session
	if err := c.DoWithContext(ctx, client.MethodDelete, &response, client.OptPath("session", id.String())); err != nil {
		return nil, err
	}

	return &response, nil
}

// UpdateSession patches the metadata for a session by ID and returns the updated session.
func (c *Client) UpdateSession(ctx context.Context, id uuid.UUID, meta schema.SessionMeta) (*schema.Session, error) {
	if id == uuid.Nil {
		return nil, fmt.Errorf("session ID cannot be nil")
	}

	httpReq, err := client.NewJSONRequestEx(http.MethodPatch, meta, client.ContentTypeAny)
	if err != nil {
		return nil, err
	}

	var response schema.Session
	if err := c.DoWithContext(ctx, httpReq, &response, client.OptPath("session", id.String())); err != nil {
		return nil, err
	}

	return &response, nil
}
