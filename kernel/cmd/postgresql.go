// Copyright 2026 David Thorpe
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"context"

	// Packages
	pg "github.com/mutablelogic/go-pg"
	server "github.com/mutablelogic/go-server"
	logger "github.com/mutablelogic/go-server/pkg/logger"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type PostgresFlags struct {
	Url      string `name:"url" env:"PG_URL" help:"PostgreSQL connection URL"`
	Password string `name:"password" env:"PG_PASSWORD" help:"PostgreSQL password"`
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Connect to the database and return a connection pool, or nil if no URL is set.
func (cmd *PostgresFlags) Connect(ctx server.Cmd) (pg.PoolConn, error) {
	if cmd.Url == "" {
		return nil, nil
	}
	opts := []pg.Opt{
		pg.WithURL(cmd.Url),
		pg.WithTracer(ctx.Tracer()),
	}
	if cmd.Password != "" {
		opts = append(opts, pg.WithPassword(cmd.Password))
	}
	opts = append(opts, pg.WithTrace(func(c context.Context, sql string, args any, err error) {
		if err != nil {
			ctx.Logger().Log(c, logger.LevelDebug, sql, "args", args, "err", err)
		} else {
			ctx.Logger().Log(c, logger.LevelTrace, sql, "args", args)
		}
	}))

	// Connect to the database, ping it
	pool, err := pg.NewPool(ctx.Context(), opts...)
	if err != nil {
		return nil, err
	} else if err := pool.Ping(ctx.Context()); err != nil {
		pool.Close()
		return nil, err
	}

	// Return success
	return pool, nil
}
