package pg

import (
	"context"
	"errors"
	"strings"
	"time"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/pkg/heartbeat/schema"
	pg "github.com/mutablelogic/go-pg"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Store is a PostgreSQL-backed store for Heartbeat records.
type Store struct {
	pg.PoolConn
}

var _ schema.Store = (*Store)(nil)

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
// PRIVATE METHODS

// markFiredWriter binds the @fired parameter for the mark_fired UPDATE.
// fired is true when there's no future occurrence (Next() returned zero time).
type markFiredWriter struct {
	fired bool
}

func (markFiredWriter) Insert(*pg.Bind) (string, error) { return "", llm.ErrNotImplemented }
func (w markFiredWriter) Update(bind *pg.Bind) error {
	bind.Set("fired", w.fired)
	return nil
}

// pgErr maps pg-library sentinel errors to llm errors understood by the HTTP handler.
func pgErr(err error) error {
	switch {
	case errors.Is(err, pg.ErrNotFound):
		return llm.ErrNotFound
	case errors.Is(err, pg.ErrNotImplemented):
		return llm.ErrNotImplemented
	case errors.Is(err, pg.ErrBadParameter):
		return llm.ErrBadParameter
	case errors.Is(err, pg.ErrNotAvailable):
		return llm.ErrConflict
	default:
		return err
	}
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

func (s *Store) Create(ctx context.Context, meta schema.HeartbeatMeta) (*schema.Heartbeat, error) {
	var response schema.Heartbeat
	if err := s.PoolConn.Insert(ctx, &response, meta); err != nil {
		return nil, err
	}
	return types.Ptr(response), nil
}

func (s *Store) Get(ctx context.Context, id string) (*schema.Heartbeat, error) {
	var response schema.Heartbeat
	if err := s.PoolConn.Get(ctx, &response, schema.HeartbeatIDSelector(id)); err != nil {
		return nil, pgErr(err)
	}
	return types.Ptr(response), nil
}

func (s *Store) Delete(ctx context.Context, id string) (*schema.Heartbeat, error) {
	var response schema.Heartbeat
	if err := s.PoolConn.Delete(ctx, &response, schema.HeartbeatIDSelector(id)); err != nil {
		return nil, pgErr(err)
	}
	return types.Ptr(response), nil
}

func (s *Store) List(ctx context.Context, includeFired bool) ([]*schema.Heartbeat, error) {
	var list schema.HeartbeatList

	// When includeFired is false, filter to only non-fired rows.
	// When true, omit the filter so all rows are returned.
	req := schema.HeartbeatListRequest{}
	if !includeFired {
		req.Fired = types.Ptr(false)
	}
	if err := s.PoolConn.List(ctx, &list, req); err != nil {
		return nil, err
	}

	// Return the list of heartbeats
	return list.Heartbeats, nil
}

func (s *Store) Update(ctx context.Context, id string, meta schema.HeartbeatMeta) (*schema.Heartbeat, error) {
	var response schema.Heartbeat
	if err := s.PoolConn.Update(ctx, &response, schema.HeartbeatIDSelector(id), meta); err != nil {
		return nil, pgErr(err)
	}
	return types.Ptr(response), nil
}

func (s *Store) Next(ctx context.Context) ([]*schema.Heartbeat, error) {
	var result []*schema.Heartbeat
	if err := s.PoolConn.Tx(ctx, func(conn pg.Conn) error {
		// List all non-fired heartbeats within the transaction.
		var list schema.HeartbeatList
		if err := conn.List(ctx, &list, schema.HeartbeatListRequest{
			Fired: types.Ptr(false),
		}); err != nil {
			return err
		}
		now := time.Now()
		for _, h := range list.Heartbeats {
			base := h.Created
			if h.LastFired != nil {
				base = h.LastFired.Add(time.Minute)
			}
			next := h.Schedule.Next(base)
			if !next.IsZero() && !next.After(now) {
				// Check if there's a future occurrence after now
				futureNext := h.Schedule.Next(now.Add(time.Minute))
				var fired schema.Heartbeat
				if err := conn.Update(ctx, &fired, schema.HeartbeatMarkFiredSelector(h.ID), markFiredWriter{fired: futureNext.IsZero()}); err != nil {
					return err
				}
				result = append(result, &fired)
			}
		}
		return nil
	}); err != nil {
		return nil, pgErr(err)
	}
	return result, nil
}
