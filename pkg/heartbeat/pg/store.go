package pg

import (
	"context"
	"strings"

	// Packages

	llm "github.com/mutablelogic/go-llm"
	heartbeat "github.com/mutablelogic/go-llm/pkg/heartbeat"
	pg "github.com/mutablelogic/go-pg"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Store is a PostgreSQL-backed store for Heartbeat records.
type Store struct {
	pg.PoolConn
}

var _ heartbeat.Store = (*Store)(nil)

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	defaultSchema = "heartbeat"
)

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// NewStore creates a new PostgreSQL-backed heartbeat store using the provided connection pool.
func NewStore(ctx context.Context, pool pg.PoolConn) (*Store, error) {
	// Parse query SQL
	queries, err := pg.NewQueries(strings.NewReader(queries))
	if err != nil {
		return nil, err
	}

	// Add the queries to the pool
	if pool == nil {
		return nil, llm.ErrBadParameter.With("pool is required")
	} else {
		// Set pool with default schema
		pool = pool.WithQueries(queries).With("schema", defaultSchema).(pg.PoolConn)

		// bootstrap the "default" database schema
		if err := bootstrap(ctx, pool, defaultSchema); err != nil {
			return nil, err
		}
	}

	// Success
	return &Store{PoolConn: pool}, nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

func bootstrap(ctx context.Context, conn pg.Conn, schema string) error {
	// Parse object SQL
	objects, err := pg.NewQueries(strings.NewReader(objects))
	if err != nil {
		return err
	}

	// Check schema exists, create if not
	if err := pg.SchemaCreate(ctx, conn, schema); err != nil {
		return err
	}

	// Iterate through object creation queries
	for _, key := range objects.Keys() {
		sql := objects.Query(key)
		if err := conn.Exec(ctx, sql); err != nil {
			return err
		}
	}

	// Success
	return nil
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

func (s *Store) Create(message string, schedule heartbeat.TimeSpec) (*heartbeat.Heartbeat, error) {
	return nil, llm.ErrNotImplemented
}

func (s *Store) Get(id string) (*heartbeat.Heartbeat, error) {
	return nil, llm.ErrNotImplemented
}

func (s *Store) Delete(id string) error {
	return llm.ErrNotImplemented
}

func (s *Store) List(includeFired bool) ([]*heartbeat.Heartbeat, error) {
	return nil, llm.ErrNotImplemented
}

func (s *Store) Update(id, message string, schedule *heartbeat.TimeSpec) (*heartbeat.Heartbeat, error) {
	return nil, llm.ErrNotImplemented
}

func (s *Store) MarkFired(id string) error {
	return llm.ErrNotImplemented
}

func (s *Store) Due() ([]*heartbeat.Heartbeat, error) {
	return nil, llm.ErrNotImplemented
}
