package heartbeat

import (
	"context"
	"fmt"
	"strings"

	// Packages
	otel "github.com/mutablelogic/go-client/pkg/otel"
	llm "github.com/mutablelogic/go-llm"
	heartbeatpg "github.com/mutablelogic/go-llm/heartbeat/pg"
	pg "github.com/mutablelogic/go-pg"
	attribute "go.opentelemetry.io/otel/attribute"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Manager owns a database connection and runs a background loop that fires due heartbeats.
// Create one with New, register an OnFire callback, then call Run in a goroutine.
type Manager struct {
	opts
	pg.PoolConn
}

var _ llm.Connector = (*Manager)(nil)

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// New creates a Manager backed by a PostgreSQL connection pool.
func New(ctx context.Context, pool pg.PoolConn, opts ...Opt) (*Manager, error) {
	self := new(Manager)

	// Apply configuration
	self.defaults()
	if err := self.apply(opts...); err != nil {
		return nil, err
	}
	if pool == nil {
		return nil, fmt.Errorf("pool is required")
	}

	// Set up the database connection and store
	queries, err := pg.NewQueries(strings.NewReader(heartbeatpg.Queries))
	if err != nil {
		return nil, fmt.Errorf("parse queries.sql: %w", err)
	}
	pool = pool.WithQueries(queries).With(
		"schema", self.schema,
		"llm", self.llmschema,
	).(pg.PoolConn)

	bootstrapCtx, endBootstrapSpan := otel.StartSpan(self.tracer, ctx, "heartbeat.manager.bootstrap",
		attribute.String("schema", self.schema),
	)
	if err := bootstrap(bootstrapCtx, pool, self.schema); err != nil {
		endBootstrapSpan(err)
		return nil, err
	} else {
		self.PoolConn = pool
	}
	endBootstrapSpan(nil)

	// Return success
	return self, nil
}

func bootstrap(ctx context.Context, conn pg.Conn, schemaName string) error {
	objects, err := pg.NewQueries(strings.NewReader(heartbeatpg.Objects))
	if err != nil {
		return fmt.Errorf("parse objects.sql: %w", err)
	}

	if err := pg.SchemaCreate(ctx, conn, schemaName); err != nil {
		return fmt.Errorf("create schema %q: %w", schemaName, err)
	}

	for _, key := range objects.Keys() {
		if err := conn.Exec(ctx, objects.Query(key)); err != nil {
			return fmt.Errorf("create object %q: %w", key, err)
		}
	}

	return nil
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// ListTools satisfies the llm.Connector interface.
func (m *Manager) ListTools(context.Context) ([]llm.Tool, error) {
	return nil, nil
}

// ListPrompts satisfies the llm.Connector interface.
func (m *Manager) ListPrompts(context.Context) ([]llm.Prompt, error) {
	return nil, nil
}

// ListResources satisfies the llm.Connector interface.
func (m *Manager) ListResources(context.Context) ([]llm.Resource, error) {
	return nil, nil
}
