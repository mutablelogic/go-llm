//go:build !client && !webhook_memory

package main

import (
	"path/filepath"

	// Packages
	store "github.com/mutablelogic/go-llm/webhook/store/sqlite"
	server "github.com/mutablelogic/go-server"
	broadcaster "github.com/mutablelogic/go-server/broadcaster"
)

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

// Backend is the store of the webhooks, which it's built with, of
// WEBHOOK_BACKEND
const Backend = "sqlite"

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// newStore returns the store of the webhooks, a SQLite database, in the
// user's data folder, which is created when it doesn't exist, which
// publishes its changes to the broadcaster
func newStore(ctx server.Cmd, b *broadcaster.Broadcaster) (*store.Store, error) {
	dir, err := ctx.DataDir()
	if err != nil {
		return nil, err
	}
	path := filepath.Join(dir, "webhook.db")
	ctx.Logger().InfoContext(ctx.Context(), "webhook store", "path", path)
	return store.New(ctx.Context(), path, store.WithTracer(ctx.Tracer()), store.WithBroadcaster(b))
}
