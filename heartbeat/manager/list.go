package heartbeat

import (
	"context"

	// Packages
	auth "github.com/djthorpe/go-auth/schema/auth"
	otel "github.com/mutablelogic/go-client/pkg/otel"
	schema "github.com/mutablelogic/go-llm/heartbeat/schema"
	pg "github.com/mutablelogic/go-pg"
	types "github.com/mutablelogic/go-server/pkg/types"
	attribute "go.opentelemetry.io/otel/attribute"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// List returns heartbeats matching the request.
// If user is non-nil, only heartbeats whose session belongs to that user are returned.
func (m *Manager) List(ctx context.Context, req schema.HeartbeatListRequest, user *auth.User) (_ *schema.HeartbeatList, err error) {
	ctx, endSpan := otel.StartSpan(m.tracer, ctx, "ListHeartbeats",
		attribute.String("req", types.Stringify(req)),
		attribute.String("user", types.Stringify(user)),
	)
	defer func() { endSpan(err) }()

	// Make the query and execute it
	result := schema.HeartbeatList{
		HeartbeatListRequest: req,
	}
	if err := m.PoolConn.With("user", user.UUID()).List(ctx, &result, req); err != nil {
		return nil, pg.NormalizeError(err)
	}
	result.OffsetLimit.Clamp(uint64(result.Count))

	// Return success
	return types.Ptr(result), nil
}
