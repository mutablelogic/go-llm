package httpclient

import (
	"context"
	"net/http"
	"uuid"

	// Packages
	client "github.com/mutablelogic/go-client"
	schema "github.com/mutablelogic/go-llm/registry/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// ListSessions returns the sessions, without their messages, most recently
// modified first, as the request selects, and how many there are
func (c *Client) ListSessions(ctx context.Context, req schema.SessionListRequest) (*schema.SessionList, error) {
	var response schema.SessionList
	if err := c.DoWithContext(ctx, client.MethodGet, &response, client.OptPath("llm", "session"), client.OptQuery(req.Query())); err != nil {
		return nil, err
	}
	return &response, nil
}

// GetSession returns the session with the identifier, with its messages
func (c *Client) GetSession(ctx context.Context, id uuid.UUID) (*schema.Session, error) {
	var response schema.Session
	if err := c.DoWithContext(ctx, client.MethodGet, &response, client.OptPath("llm", "session", id.String())); err != nil {
		return nil, err
	}
	return &response, nil
}

// CreateSession creates a session with the settings, and no messages, and
// returns it
func (c *Client) CreateSession(ctx context.Context, meta schema.SessionMeta) (*schema.Session, error) {
	req, err := client.NewJSONRequest(meta)
	if err != nil {
		return nil, err
	}
	var response schema.Session
	if err := c.DoWithContext(ctx, req, &response, client.OptPath("llm", "session")); err != nil {
		return nil, err
	}
	return &response, nil
}

// UpdateSession changes the settings of the session with the identifier,
// as SessionMeta.Merge does, so what the update doesn't set is kept, and
// returns it
func (c *Client) UpdateSession(ctx context.Context, id uuid.UUID, meta schema.SessionMeta) (*schema.Session, error) {
	req, err := client.NewJSONRequestEx(http.MethodPatch, meta, types.ContentTypeAny)
	if err != nil {
		return nil, err
	}
	var response schema.Session
	if err := c.DoWithContext(ctx, req, &response, client.OptPath("llm", "session", id.String())); err != nil {
		return nil, err
	}
	return &response, nil
}

// DeleteSession deletes the session with the identifier, and returns it
func (c *Client) DeleteSession(ctx context.Context, id uuid.UUID) (*schema.Session, error) {
	var response schema.Session
	if err := c.DoWithContext(ctx, client.MethodDelete, &response, client.OptPath("llm", "session", id.String())); err != nil {
		return nil, err
	}
	return &response, nil
}
