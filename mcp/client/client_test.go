package client_test

import (
	"context"
	"errors"
	"net/http/httptest"
	"testing"
	"time"

	// Packages
	client "github.com/mutablelogic/go-llm/mcp/client"
	mock "github.com/mutablelogic/go-llm/mcp/mock"
	server "github.com/mutablelogic/go-llm/mcp/server"
	schema "github.com/mutablelogic/go-llm/kernel/schema"
)

///////////////////////////////////////////////////////////////////////////////
// HELPERS

func newTestServer(t *testing.T, srvName, srvVersion string, tools ...*mock.MockTool) (*httptest.Server, *client.Client) {
	t.Helper()
	srv, err := server.New(srvName, srvVersion)
	if err != nil {
		t.Fatal(err)
	}
	for _, mt := range tools {
		if err := srv.AddTools(mt); err != nil {
			t.Fatal(err)
		}
	}
	ts := httptest.NewServer(srv.Handler())
	c, err := client.New(ts.URL, "test-client", "1.0.0")
	if err != nil {
		ts.Close()
		t.Fatal(err)
	}
	t.Cleanup(ts.Close)
	return ts, c
}

// runClient starts c.Run in a background goroutine and returns a cancel func.
// Polls until ListTools stops returning ErrServiceUnavailable (up to 2s).
func runClient(t *testing.T, c *client.Client) context.CancelFunc {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	go func() { _ = c.Run(ctx) }()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		_, err := c.ListTools(context.Background())
		if !errors.Is(err, schema.ErrServiceUnavailable) {
			return cancel
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	t.Fatal("client did not connect within 2s")
	return cancel
}
