//go:build !client && webhook_memory

package main

import (
	// Packages
	store "github.com/mutablelogic/go-llm/webhook/store/sqlite"
	server "github.com/mutablelogic/go-server"
	broadcaster "github.com/mutablelogic/go-server/broadcaster"
	sqlite "github.com/mutablelogic/go-sqlite"
)

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

// Backend is the store of the webhooks, which it's built with, of
// WEBHOOK_BACKEND
const Backend = "memory"

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// newStore returns the store of the webhooks, in memory, so they aren't
// kept when the server stops, which publishes its changes to the
// broadcaster
func newStore(ctx server.Cmd, b *broadcaster.Broadcaster) (*store.Store, error) {
	return store.New(ctx.Context(), sqlite.Memory, store.WithTracer(ctx.Tracer()), store.WithBroadcaster(b))
}
