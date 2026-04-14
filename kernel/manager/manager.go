package manager

import (
	"context"
	"fmt"
	"strings"
	"time"

	// Packages
	otel "github.com/mutablelogic/go-client/pkg/otel"
	schema "github.com/mutablelogic/go-llm/kernel/schema"
	providerregistry "github.com/mutablelogic/go-llm/provider/registry"
	toolkit "github.com/mutablelogic/go-llm/toolkit"
	pg "github.com/mutablelogic/go-pg"
	broadcaster "github.com/mutablelogic/go-pg/pkg/broadcaster"
	attribute "go.opentelemetry.io/otel/attribute"
	metric "go.opentelemetry.io/otel/metric"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type Manager struct {
	manageropt
	pg.PoolConn
	*providerregistry.Registry
	toolkit.Toolkit
	broadcaster broadcaster.Broadcaster
	sessionfeed *SessionFeed
	delegate    *delegate
}

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

func New(ctx context.Context, name, version string, pool pg.PoolConn, opts ...Opt) (*Manager, error) {
	// Set default values
	self := new(Manager)
	self.defaults(name, version)

	// Check arguments
	if pool == nil {
		return nil, fmt.Errorf("pool is required")
	}

	// Apply options
	if err := self.apply(opts...); err != nil {
		return nil, err
	}

	// Parse and register named queries so bind.Query(...) can resolve them.
	queries, err := pg.NewQueries(strings.NewReader(schema.Queries))
	if err != nil {
		return nil, fmt.Errorf("parse queries.sql: %w", err)
	} else {
		pool = pool.WithQueries(queries).With(
			"schema", self.llmschema,
			"auth", self.authschema,
			"channel", self.channel,
		).(pg.PoolConn)
	}

	// Create objects in the database schema. This is not done in a transaction
	bootstrapCtx, endBootstrapSpan := otel.StartSpan(self.tracer, ctx, "llmmanager.bootstrap",
		attribute.String("schema", self.llmschema),
	)
	if err := bootstrap(bootstrapCtx, pool, self.llmschema); err != nil {
		endBootstrapSpan(err)
		return nil, err
	} else {
		endBootstrapSpan(nil)
		self.PoolConn = pool
	}

	// Set up notifications of table change if requested
	if self.channel != "" {
		if broadcaster, err := broadcaster.NewBroadcaster(pool, self.channel); err != nil {
			return nil, err
		} else {
			self.broadcaster = broadcaster
		}
	}

	// Create the provider registry
	if registry := providerregistry.New(self.clientopts...); registry == nil {
		return nil, fmt.Errorf("unable to create provider registry")
	} else {
		self.Registry = registry
	}

	// Create a connector delegate, which receives notifications of connector changes
	self.delegate = NewDelegate(self.name, self.version, self.connectors, self.runAgent, self.clientopts...)

	// Create a session feed, which updates listening sessions when new messages are added
	if sessionfeed, err := NewSessionFeed(ctx, pool, time.Second); err != nil {
		return nil, err
	} else {
		self.sessionfeed = sessionfeed
	}

	// TEST TODO
	// Register metrics after the registry has been initialized so callbacks can
	// safely read manager state during collection.
	if err := self.registerMetrics(); err != nil {
		return nil, err
	}

	// Return success
	return self, nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

func bootstrap(ctx context.Context, conn pg.Conn, schemaName string) error {
	// Get all objects
	objects, err := pg.NewQueries(strings.NewReader(schema.Objects))
	if err != nil {
		return fmt.Errorf("parse objects.sql: %w", err)
	}

	// Create the schema
	if err := pg.SchemaCreate(ctx, conn, schemaName); err != nil {
		return fmt.Errorf("create schema %q: %w", schemaName, err)
	}

	// Create all objects - not in a transaction
	for _, key := range objects.Keys() {
		if err := conn.Exec(ctx, objects.Query(key)); err != nil {
			return fmt.Errorf("create object %q: %w", key, err)
		}
	}

	// Return success
	return nil
}

func (m *Manager) registerMetrics() error {
	if m.metrics == nil || m.Registry == nil {
		return nil
	}

	gauge, err := m.metrics.Float64ObservableGauge(
		"llmmanager.providers",
		metric.WithDescription("Number of providers loaded in the in-memory registry"),
	)
	if err != nil {
		return fmt.Errorf("register llmmanager.providers gauge: %w", err)
	}

	if _, err := m.metrics.RegisterCallback(func(ctx context.Context, observer metric.Observer) error {
		observer.ObserveFloat64(gauge, float64(m.Registry.Count()))
		return nil
	}, gauge); err != nil {
		return fmt.Errorf("register llmmanager.providers callback: %w", err)
	}

	return nil
}
