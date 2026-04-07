package client_test

import (
	"testing"

	// Packages
	mock "github.com/mutablelogic/go-llm/mcp/mock"
)

// Test_session_001: ListTools returns all tools registered on the server.
func Test_session_001(t *testing.T) {
	_, c := newTestServer(t, "list-server", "1.0.0",
		&mock.MockTool{Name_: "alpha", Description_: "Alpha tool"},
		&mock.MockTool{Name_: "beta", Description_: "Beta tool"},
	)
	cancel := runClient(t, c)
	defer cancel()

	tools, err := c.ListTools(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if len(tools) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(tools))
	}
	names := map[string]bool{tools[0].Name(): true, tools[1].Name(): true}
	if !names["alpha"] || !names["beta"] {
		t.Errorf("unexpected tool names: %v", names)
	}
}

// Test_session_002: ServerInfo returns the server name, version and a protocol string.
func Test_session_002(t *testing.T) {
	_, c := newTestServer(t, "info-server", "9.8.7")
	cancel := runClient(t, c)
	defer cancel()

	name, version, protocol := c.ServerInfo()
	if name != "info-server" {
		t.Errorf("expected name %q, got %q", "info-server", name)
	}
	if version != "9.8.7" {
		t.Errorf("expected version %q, got %q", "9.8.7", version)
	}
	if protocol == "" {
		t.Error("expected non-empty protocol version")
	}
}

// Test_session_003: ServerInfo returns empty strings before Run connects.
func Test_session_003(t *testing.T) {
	_, c := newTestServer(t, "info-server", "1.0.0")
	name, version, protocol := c.ServerInfo()
	if name != "" || version != "" || protocol != "" {
		t.Errorf("expected empty ServerInfo before connect, got %q %q %q", name, version, protocol)
	}
}
