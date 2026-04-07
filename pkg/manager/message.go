package manager

import (
	"context"

	// Packages
	auth "github.com/djthorpe/go-auth/schema/auth"
	uuid "github.com/google/uuid"
	otel "github.com/mutablelogic/go-client/pkg/otel"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	pg "github.com/mutablelogic/go-pg"
	types "github.com/mutablelogic/go-server/pkg/types"
	attribute "go.opentelemetry.io/otel/attribute"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// ListMessages returns messages for a session, optionally filtered by request fields.
// If user is non-nil, the session must be owned by that user.
func (m *Manager) ListMessages(ctx context.Context, session uuid.UUID, req schema.MessageListRequest, user *auth.User) (_ *schema.MessageList, err error) {
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
	conn := m.PoolConn.With("session", session)
	if user != nil {
		conn = conn.With("user", user.UUID())
	}
	if err := conn.List(ctx, &result, req); err != nil {
		return nil, pg.NormalizeError(err)
	}
	result.OffsetLimit.Clamp(uint64(result.Count))

	// Return success
	return types.Ptr(result), nil
}
