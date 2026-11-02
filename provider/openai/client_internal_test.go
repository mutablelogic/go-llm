package openai

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	// Packages
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

func TestPingAfterInterval(t *testing.T) {
	// A server which counts its requests
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"object":"list","data":[]}`))
	}))
	t.Cleanup(server.Close)
	c, err := New(server.URL+"/v1", "")
	require.NoError(t, err)
	ctx := context.Background()

	// The API is pinged again once the ping interval has passed
	require.NoError(t, c.Ping(ctx))
	require.NoError(t, c.Ping(ctx))
	assert.EqualValues(t, 1, requests.Load())
	c.lastPing = time.Now().Add(-PingInterval)
	require.NoError(t, c.Ping(ctx))
	assert.EqualValues(t, 2, requests.Load())
}
