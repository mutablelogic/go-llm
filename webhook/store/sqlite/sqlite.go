// Package sqlite is a store of webhooks, and their calls, in a SQLite
// database, which is created when it doesn't exist, or in memory, of
// sqlite.Memory, such as for tests
package sqlite

import (
	"context"
	"errors"
	"strings"

	// Packages
	otel "github.com/mutablelogic/go-client/pkg/otel"
	llm "github.com/mutablelogic/go-llm"
	store "github.com/mutablelogic/go-llm/webhook/store"
	schema "github.com/mutablelogic/go-llm/webhook/store/sqlite/schema"
	sqlite "github.com/mutablelogic/go-sqlite"
	ncruces "github.com/mutablelogic/go-sqlite/ncruces"
	attribute "go.opentelemetry.io/otel/attribute"
	trace "go.opentelemetry.io/otel/trace"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Store is a store of webhooks, and their calls, in a SQLite database
type Store struct {
	conn        sqlite.PoolConn // The pool's connection, with the schema's queries
	tracer      trace.Tracer
	broadcaster sqlite.Publisher
}

// Opt is an option of the store
type Opt func(*Store) error

var _ store.Store = (*Store)(nil)

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// New returns a store of the path of its database file, such as
// /data/llm/webhook.db, or sqlite.Memory, which is created when it doesn't
// exist, with its schema
func New(ctx context.Context, path string, opts ...Opt) (_ *Store, err error) {
	self := new(Store)
	for _, opt := range opts {
		if err := opt(self); err != nil {
			return nil, err
		}
	}
	if strings.TrimSpace(path) == "" {
		return nil, llm.ErrBadParameter.With("the webhook store needs its path")
	}

	// Trace opening it, and creating its schema
	ctx, end := otel.StartSpan(self.tracer, ctx, "webhook.store.sqlite.New", attribute.String("arg.path", path))
	defer func() { end(err) }()

	// Open the database
	var dbopts []sqlite.Opt
	if self.tracer != nil {
		dbopts = append(dbopts, sqlite.WithTracer(self.tracer))
	}
	if self.broadcaster != nil {
		dbopts = append(dbopts, sqlite.WithBroadcaster(self.broadcaster, store.WebhookTable, store.CallTable), sqlite.WithSchemaName(store.Schema))
	}
	pool, err := ncruces.Open(ctx, path, dbopts...)
	if err != nil {
		return nil, storeErr(err)
	}

	// Create its schema, and bind its queries
	queries, err := schema.NewQueries()
	if err != nil {
		return nil, errors.Join(err, pool.Close())
	} else if err := schema.Create(ctx, pool); err != nil {
		return nil, errors.Join(storeErr(err), pool.Close())
	}
	self.conn = pool.WithQueries(queries).(sqlite.PoolConn)
	return self, nil
}

// WithBroadcaster publishes the webhooks, and calls, which are created,
// updated and deleted, once they're committed, as changes of the schema
// store.Schema, and the tables store.WebhookTable, and store.CallTable,
// whose rows are their serial numbers, including the calls which are deleted with their webhook, and
// those which are deleted, as they're old
func WithBroadcaster(b sqlite.Publisher) Opt {
	return func(s *Store) error {
		if b == nil {
			return llm.ErrBadParameter.With("missing broadcaster")
		}
		s.broadcaster = b
		return nil
	}
}

// WithTracer traces the store's methods, and their queries
func WithTracer(tracer trace.Tracer) Opt {
	return func(s *Store) error {
		s.tracer = tracer
		return nil
	}
}

// Close closes the database
func (self *Store) Close() error {
	return self.conn.Close()
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// span starts a span of a method of the store, such as
// webhook.store.sqlite.CreateWebhook, with its arguments
func (self *Store) span(ctx context.Context, method string, attrs ...attribute.KeyValue) (context.Context, func(error)) {
	return otel.StartSpan(self.tracer, ctx, "webhook.store.sqlite."+method, attrs...)
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// storeErr returns the error of the database as go-llm's, of its kind, such
// as ErrNotFound, or ErrConflict
func storeErr(err error) error {
	var llmErr llm.Err
	switch {
	case err == nil:
		return nil
	case errors.As(err, &llmErr), errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return err
	case errors.Is(err, sqlite.ErrNotFound):
		return llm.ErrNotFound.With(err.Error())
	case errors.Is(err, sqlite.ErrConflict):
		return llm.ErrConflict.With(err.Error())
	case errors.Is(err, sqlite.ErrBadParameter):
		return llm.ErrBadParameter.With(err.Error())
	case errors.Is(err, sqlite.ErrNotAvailable):
		return llm.ErrServiceUnavailable.With(err.Error())
	}
	return llm.ErrInternalServerError.With(err.Error())
}
