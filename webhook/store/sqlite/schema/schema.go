// Package schema is the schema of the SQLite store of webhooks: its tables,
// its queries, and the types which are read from, and written to, them
package schema

import (
	"context"
	_ "embed"
	"strings"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	sqlite "github.com/mutablelogic/go-sqlite"
)

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// Version is the version of the schema, of the database's user_version
	Version = 1

	// ListLimit is the most rows which are listed at once
	ListLimit = 100
)

var (
	//go:embed objects.sql
	objects string

	//go:embed queries.sql
	queries string
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC FUNCTIONS

// Create creates the schema, in a transaction, when the database hasn't
// one, and sets its version. A database whose schema is newer is an error.
func Create(ctx context.Context, conn sqlite.Conn) error {
	objects, err := sqlite.NewQueries(strings.NewReader(objects))
	if err != nil {
		return err
	}
	return conn.Tx(ctx, func(conn sqlite.Conn) error {
		version, err := sqlite.Version(ctx, conn)
		if err != nil {
			return err
		} else if version > Version {
			return llm.ErrNotImplemented.Withf("the webhook store's schema version %d is newer than %d", version, Version)
		} else if version == Version {
			return nil
		}
		for _, key := range objects.Keys() {
			if err := conn.With(sqlite.TraceSpanNameArg, key).Exec(ctx, objects.Query(key)); err != nil {
				return err
			}
		}
		return sqlite.SetVersion(ctx, conn, Version)
	})
}

// NewQueries returns the queries, which a connection is bound to, with
// WithQueries, so the types' selectors and writers use them
func NewQueries() (*sqlite.Queries, error) {
	return sqlite.NewQueries(strings.NewReader(queries))
}
