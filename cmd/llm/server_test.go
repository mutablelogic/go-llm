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

func Test_FiresAgain(t *testing.T) {
	// A session is kept when it has a heartbeat which fires again, however
	// long since it changed, and not otherwise
	ctx := context.Background()
	h, err := heartbeat.New(heartbeat.NewStore())
	require.NoError(t, err)
	keep := firesAgain(h)
	reminded, quiet := uuid.New(), uuid.New()
	at := time.Now().Add(21 * 24 * time.Hour)
	_, err = h.Create(ctx, heartbeatschema.HeartbeatMeta{Title: "Call", Prompt: "Remind David to call Anne", Session: reminded, At: &at})
	require.NoError(t, err)

	got, err := keep(ctx, reminded)
	require.NoError(t, err)
	assert.True(t, got)
	got, err = keep(ctx, quiet)
	require.NoError(t, err)
	assert.False(t, got)
}
