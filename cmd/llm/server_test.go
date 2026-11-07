//go:build !client

package main

import (
	"context"
	"testing"
	"time"
	"uuid"

	// Packages
	heartbeat "github.com/mutablelogic/go-llm/heartbeat"
	heartbeatschema "github.com/mutablelogic/go-llm/heartbeat/schema"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

func Test_HasHeartbeats(t *testing.T) {
	// A session is kept when it has a heartbeat, however long since it
	// changed: one which fires again, or one which fired, which may be
	// waiting to run, or running, in it, and not otherwise
	ctx := context.Background()
	store := heartbeat.NewStore()
	h, err := heartbeat.New(store)
	require.NoError(t, err)
	keep := hasHeartbeats(h)
	reminded, fired, quiet := uuid.New(), uuid.New(), uuid.New()
	at := time.Now().Add(21 * 24 * time.Hour)
	_, err = h.Create(ctx, heartbeatschema.HeartbeatMeta{Title: "Call", Prompt: "Remind David to call Anne", Session: reminded, At: &at})
	require.NoError(t, err)
	now := time.Now()
	require.NoError(t, store.Create(ctx, heartbeatschema.Heartbeat{ID: uuid.New(), HeartbeatMeta: heartbeatschema.HeartbeatMeta{Title: "Done", Prompt: "Remind David", Session: fired, At: &now}, Fired: &now}))

	for id, want := range map[uuid.UUID]bool{reminded: true, fired: true, quiet: false} {
		got, err := keep(ctx, id)
		require.NoError(t, err)
		assert.Equal(t, want, got, id)
	}
}
