package httpclient

import (
	"context"
	"fmt"
	"net/http"

	// Packages
	client "github.com/mutablelogic/go-client"
	opt "github.com/mutablelogic/go-llm/pkg/opt"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// ListSessions returns a list of all sessions.
// Use WithLimit and WithOffset to paginate results.
func (c *Client) ListSessions(ctx context.Context, opts ...opt.Opt) (*schema.ListSessionResponse, error) {
	// Apply options
	o, err := opt.Apply(opts...)
	if err != nil {
		return nil, err
	}

	// Create request
	req := client.NewRequest()
	reqOpts := []client.RequestOpt{client.OptPath("session")}
	if q := o.Query(opt.LimitKey, opt.OffsetKey); len(q) > 0 {
		reqOpts = append(reqOpts, client.OptQuery(q))
	}

	// Perform request
	var response schema.ListSessionResponse
	if err := c.DoWithContext(ctx, req, &response, reqOpts...); err != nil {
		return nil, err
	}

	// Return the response
	return &response, nil
}

// GetSession retrieves a session by ID.
func (c *Client) GetSession(ctx context.Context, id string) (*schema.Session, error) {
	if id == "" {
		return nil, fmt.Errorf("session ID cannot be empty")
	}

	// Create request
	req := client.NewRequest()
	reqOpts := []client.RequestOpt{client.OptPath("session", id)}

	// Perform request
	var response schema.Session
	if err := c.DoWithContext(ctx, req, &response, reqOpts...); err != nil {
		return nil, err
	}

	// Return the response
	return &response, nil
}

// CreateSession creates a new session with the given metadata.
func (c *Client) CreateSession(ctx context.Context, meta schema.SessionMeta) (*schema.Session, error) {
	// Create request
	req, err := client.NewJSONRequest(meta)
	if err != nil {
		return nil, err
	}
	reqOpts := []client.RequestOpt{client.OptPath("session")}

	// Perform request
	var response schema.Session
	if err := c.DoWithContext(ctx, req, &response, reqOpts...); err != nil {
		return nil, err
	}

	// Return the response
	return &response, nil
}

// DeleteSession deletes a session by ID.
func (c *Client) DeleteSession(ctx context.Context, id string) error {
	if id == "" {
		return fmt.Errorf("session ID cannot be empty")
	}

	// Create request
	reqOpts := []client.RequestOpt{client.OptPath("session", id)}

	// Perform request
	if err := c.DoWithContext(ctx, client.MethodDelete, nil, reqOpts...); err != nil {
		return err
	}

	// Return success
	return nil
}

// UpdateSession updates a session's metadata by ID.
func (c *Client) UpdateSession(ctx context.Context, id string, meta schema.SessionMeta) (*schema.Session, error) {
	if id == "" {
		return nil, fmt.Errorf("session ID cannot be empty")
	}

	// Create request
	req, err := client.NewJSONRequestEx(http.MethodPatch, meta, "")
	if err != nil {
		return nil, err
	}
	reqOpts := []client.RequestOpt{client.OptPath("session", id)}

	// Perform request
	var response schema.Session
	if err := c.DoWithContext(ctx, req, &response, reqOpts...); err != nil {
		return nil, err
	}

	// Return the response
	return &response, nil
}
