package server_test

import (
	"context"
	"testing"

	// Packages
	client "github.com/mutablelogic/go-llm/pkg/mcp/client"
	server "github.com/mutablelogic/go-llm/pkg/mcp/server"
	servertest "github.com/mutablelogic/go-llm/pkg/mcp/server/test"
)

func TestServerProbe(t *testing.T) {
	// Create the MCP server.
	srv, err := server.New("test-server", "1.0.0")
	if err != nil {
		t.Fatal(err)
	}

	// Wrap it in an httptest.Server so we get a local URL.
	ts, _ := servertest.Connect(t, srv)
	_ = ts

	// Connect a client and probe the server.
	c, err := client.New(ts.URL, "test-client", "1.0.0", nil)
	if err != nil {
		t.Fatal(err)
	}

	state, err := c.Probe(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	if state.Name == nil || *state.Name != "test-server" {
		t.Fatalf("expected server name %q, got %v", "test-server", state.Name)
	}
	if state.Version == nil || *state.Version != "1.0.0" {
		t.Fatalf("expected server version %q, got %v", "1.0.0", state.Version)
	}
}
