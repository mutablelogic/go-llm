package manager

import (
	"context"

	// Packages
	auth "github.com/djthorpe/go-auth/schema/auth"
	otel "github.com/mutablelogic/go-client/pkg/otel"
	schema "github.com/mutablelogic/go-llm/kernel/schema"
	pg "github.com/mutablelogic/go-pg"
	types "github.com/mutablelogic/go-server/pkg/types"
	attribute "go.opentelemetry.io/otel/attribute"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// ListMessages returns messages for a single session, optionally filtered by request fields.
// If user is non-nil, the session must be owned by that user.
func (m *Manager) ListMessages(ctx context.Context, req schema.MessageListRequest, user *auth.User) (_ *schema.MessageList, err error) {
	if len(req.Sessions) != 1 {
		return nil, schema.ErrBadParameter.With("exactly one message session is required")
	}
	session := req.Sessions[0]

	ctx, endSpan := otel.StartSpan(m.tracer, ctx, "ListMessages",
		attribute.String("session", session.String()),
		attribute.String("req", req.String()),
		attribute.String("user", types.Stringify(user)),
	)
	defer func() { endSpan(err) }()

	if _, err := m.GetSession(ctx, session, user); err != nil {
		return nil, err
	}

	result := schema.MessageList{MessageListRequest: req}
	if err := m.PoolConn.With("user", user.UUID()).List(ctx, &result, req); err != nil {
		return nil, pg.NormalizeError(err)
	}
	result.OffsetLimit.Clamp(uint64(result.Count))

	// Return success
	return types.Ptr(result), nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// ListMessagesForSession returns messages for many sessions.
func (m *Manager) listSessionMessages(ctx context.Context, req schema.MessageListRequest) (_ *schema.MessageList, err error) {
	ctx, endSpan := otel.StartSpan(m.tracer, ctx, "ListSessionMessages",
		attribute.String("sessions", types.Stringify(req.Sessions)),
		attribute.Int64("last", int64(req.Last)),
		attribute.String("req", req.String()),
	)
	defer func() { endSpan(err) }()

	result := schema.MessageList{MessageListRequest: req}
	if err := m.PoolConn.List(ctx, &result, req); err != nil {
		return nil, pg.NormalizeError(err)
	}
	result.OffsetLimit.Clamp(uint64(result.Count))

	// Return success
	return types.Ptr(result), nil
}
