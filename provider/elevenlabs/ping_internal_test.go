package elevenlabs

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

func TestPing(t *testing.T) {
	// A server which counts its requests
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[]`))
	}))
	t.Cleanup(server.Close)
	c, err := New(server.URL+"/v1", "key")
	require.NoError(t, err)

	// A ping which ends as its caller gives up isn't kept
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	assert.Error(t, c.Ping(cancelled))
	assert.True(t, c.lastPing.IsZero(), "the cancelled ping isn't kept")
	assert.EqualValues(t, 0, requests.Load())

	// The next is, and is used, until the ping interval has passed
	require.NoError(t, c.Ping(context.Background()))
	require.NoError(t, c.Ping(context.Background()))
	assert.EqualValues(t, 1, requests.Load())
	c.lastPing = time.Now().Add(-PingInterval)
	require.NoError(t, c.Ping(context.Background()))
	assert.EqualValues(t, 2, requests.Load())
}
