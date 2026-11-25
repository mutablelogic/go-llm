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
	Version = 3

	// ListLimit is the most rows which are listed at once
	ListLimit = 100
)

// upgrades are the statements which upgrade a schema, from each version to
// the next, such as from version 1, to 2. The calls which a database has,
// as it's upgraded to version 3, are seen, so they aren't all handled then.
var upgrades = map[int64][]string{
	1: {"ALTER TABLE call ADD COLUMN test BOOLEAN NOT NULL DEFAULT 0"},
	2: {
		"ALTER TABLE call ADD COLUMN seen_at TIMESTAMP",
		"UPDATE call SET seen_at = received_at WHERE error IS NULL",
		"CREATE INDEX IF NOT EXISTS call_pending ON call (id) WHERE error IS NULL AND seen_at IS NULL",
	},
}

var (
	//go:embed objects.sql
	objects string

	//go:embed queries.sql
	queries string
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC FUNCTIONS

// Create creates the schema, in a transaction, when the database hasn't
// one, or upgrades it, when it's older, and sets its version. A database
// whose schema is newer is an error.
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
		if version > 0 {
			return upgrade(ctx, conn, version)
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

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// upgrade upgrades the schema, from the version, to the current one
func upgrade(ctx context.Context, conn sqlite.Conn, version int64) error {
	for ; version < Version; version++ {
		for _, statement := range upgrades[version] {
			if err := conn.With(sqlite.TraceSpanNameArg, "schema.upgrade").Exec(ctx, statement); err != nil {
				return err
			}
		}
	}
	return sqlite.SetVersion(ctx, conn, Version)
}
