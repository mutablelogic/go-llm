package memory

import (
	"context"

	// Packages
	uuid "github.com/google/uuid"
	otel "github.com/mutablelogic/go-client/pkg/otel"
	schema "github.com/mutablelogic/go-llm/pkg/schema/memory"
	pg "github.com/mutablelogic/go-pg"
	types "github.com/mutablelogic/go-server/pkg/types"
	attribute "go.opentelemetry.io/otel/attribute"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// CreateMemory validates and persists a new memory entry.
func (m *Manager) CreateMemory(ctx context.Context, insert schema.MemoryInsert) (_ *schema.Memory, err error) {
	ctx, endSpan := otel.StartSpan(m.tracer, ctx, "CreateMemory",
		attribute.String("session", insert.Session.String()),
		attribute.String("key", insert.Key),
	)
	defer func() { endSpan(err) }()

	var result schema.Memory
	if err := m.PoolConn.Tx(ctx, func(conn pg.Conn) error {
		return conn.Insert(ctx, &result, insert)
	}); err != nil {
		return nil, pg.NormalizeError(err)
	}

	return &result, nil
}

// GetMemory returns a single memory entry by session and key.
func (m *Manager) GetMemory(ctx context.Context, session uuid.UUID, key string) (_ *schema.Memory, err error) {
	ctx, endSpan := otel.StartSpan(m.tracer, ctx, "GetMemory",
		attribute.String("session", session.String()),
		attribute.String("key", key),
	)
	defer func() { endSpan(err) }()

	var result schema.Memory
	if err := m.PoolConn.Get(ctx, &result, schema.MemorySelector{Session: session, Key: key}); err != nil {
		return nil, pg.NormalizeError(err)
	}

	return &result, nil
}

// ListMemory returns memory entries matching the request filters.
func (m *Manager) ListMemory(ctx context.Context, req schema.MemoryListRequest) (_ *schema.MemoryList, err error) {
	ctx, endSpan := otel.StartSpan(m.tracer, ctx, "ListMemory",
		attribute.String("req", types.Stringify(req)),
	)
	defer func() { endSpan(err) }()

	result := schema.MemoryList{MemoryListRequest: req}
	if err := m.PoolConn.List(ctx, &result, req); err != nil {
		return nil, pg.NormalizeError(err)
	}
	result.OffsetLimit.Clamp(uint64(result.Count))

	return &result, nil
}
