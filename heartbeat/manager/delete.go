package heartbeat

import (
	"context"

	// Packages
	uuid "github.com/google/uuid"
	otel "github.com/mutablelogic/go-client/pkg/otel"
	schema "github.com/mutablelogic/go-llm/heartbeat/schema"
	pg "github.com/mutablelogic/go-pg"
	types "github.com/mutablelogic/go-server/pkg/types"
	attribute "go.opentelemetry.io/otel/attribute"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Delete removes a heartbeat schedule.
func (m *Manager) Delete(ctx context.Context, session uuid.UUID, heartbeat uuid.UUID) (_ *schema.Heartbeat, err error) {
	ctx, endSpan := otel.StartSpan(m.tracer, ctx, "DeleteHeartbeat",
		attribute.String("session", types.Stringify(session)),
		attribute.String("heartbeat", types.Stringify(heartbeat)),
	)
	defer func() { endSpan(err) }()

	// Delete the heartbeat schedule
	var result schema.Heartbeat
	if err := m.PoolConn.Tx(ctx, func(conn pg.Conn) error {
		return conn.With("session", session).Delete(ctx, &result, schema.HeartbeatSelector(heartbeat))
	}); err != nil {
		return nil, pg.NormalizeError(err)
	}

	return types.Ptr(result), nil
}
