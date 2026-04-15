package heartbeat

import (
	"context"

	// Packages
	uuid "github.com/google/uuid"
	otel "github.com/mutablelogic/go-client/pkg/otel"
	heartbeat "github.com/mutablelogic/go-llm/heartbeat/schema"
	pg "github.com/mutablelogic/go-pg"
	types "github.com/mutablelogic/go-server/pkg/types"
	attribute "go.opentelemetry.io/otel/attribute"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Create validates a request, converts the schedule into a TimeSpec, and inserts a new heartbeat.
func (m *Manager) Create(ctx context.Context, session uuid.UUID, req heartbeat.HeartbeatMeta) (_ *heartbeat.Heartbeat, err error) {
	ctx, endSpan := otel.StartSpan(m.tracer, ctx, "CreateHeartbeat",
		attribute.String("req", types.Stringify(req)),
	)
	defer func() { endSpan(err) }()

	// Insert a new session
	var result heartbeat.Heartbeat
	if err := m.PoolConn.Tx(ctx, func(conn pg.Conn) error {
		return conn.With("session", session).Insert(ctx, &result, req)
	}); err != nil {
		return nil, pg.NormalizeError(err)
	}

	return types.Ptr(result), nil
}
