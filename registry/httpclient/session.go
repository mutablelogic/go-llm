package httpclient

import (
	"context"
	"net/http"
	"uuid"

	// Packages
	client "github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
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

// GetSession returns the session with the identifier, without its messages,
// which ListSessionMessages lists
func (c *Client) GetSession(ctx context.Context, id uuid.UUID) (*schema.Session, error) {
	var response schema.Session
	if err := c.DoWithContext(ctx, client.MethodGet, &response, client.OptPath("llm", "session", id.String())); err != nil {
		return nil, err
	}
	return &response, nil
}

// ListSessionMessages returns the messages of the session with the
// identifier, oldest first, as the request selects, and how many there are
func (c *Client) ListSessionMessages(ctx context.Context, id uuid.UUID, req schema.MessageListRequest) (*schema.MessageList, error) {
	var response schema.MessageList
	if err := c.DoWithContext(ctx, client.MethodGet, &response, client.OptPath("llm", "session", id.String(), "messages"), client.OptQuery(req.Query())); err != nil {
		return nil, err
	}
	return &response, nil
}

// ForgetSessionMessages forgets the messages of the session with the
// identifier, keeping its settings, and returns it
func (c *Client) ForgetSessionMessages(ctx context.Context, id uuid.UUID) (*schema.Session, error) {
	var response schema.Session
	if err := c.DoWithContext(ctx, client.MethodDelete, &response, client.OptPath("llm", "session", id.String(), "messages")); err != nil {
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

// SetSessionStatus sets the status of the session with the identifier, to
// typing, as someone types a message, which it is for a while after, or
// idle, when they stop, and returns the session, with its status
func (c *Client) SetSessionStatus(ctx context.Context, id uuid.UUID, status schema.SessionStatus) (*schema.Session, error) {
	req, err := client.NewJSONRequestEx(http.MethodPut, schema.SessionStatusRequest{Status: status}, types.ContentTypeAny)
	if err != nil {
		return nil, err
	}
	var response schema.Session
	if err := c.DoWithContext(ctx, req, &response, client.OptPath("llm", "session", id.String(), "status")); err != nil {
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

// Chat adds the messages to the session with the identifier, which its model
// replies to, running the tools it calls, and returns what was added, with
// the model's reply, and the tokens used. When the stream function isn't
// nil, the reply is streamed, and the function is called with each piece of
// its text and thinking, and the tools' log lines, as they're generated. The
// request isn't timed out, as a model can take a while to reply, but is
// cancelled with the context.
func (c *Client) Chat(ctx context.Context, id uuid.UUID, req schema.ChatRequest, stream llm.GeneratorStreamFn) (*schema.ChatResponse, error) {
	if len(req.Messages) == 0 {
		return nil, llm.ErrBadParameter.With("missing messages")
	}
	body, err := client.NewJSONRequest(req)
	if err != nil {
		return nil, err
	}

	// Without streaming, what was added
	if stream == nil {
		var response schema.ChatResponse
		if err := c.DoWithContext(ctx, body, &response, client.OptPath("llm", "session", id.String()), client.OptNoTimeout()); err != nil {
			return nil, err
		}
		return &response, nil
	}

	// With streaming, the pieces of the reply, and then what was added, or
	// the error
	return doStream[schema.ChatResponse](ctx, c, body, func(event client.TextStreamEvent) error {
		return streamEvent(event, stream)
	}, "llm", "session", id.String())
}
