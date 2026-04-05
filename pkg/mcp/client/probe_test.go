package client_test

import (
	"testing"

	// Packages
	mock "github.com/mutablelogic/go-llm/pkg/mcp/mock"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
)

// Test_probe_001: Probe returns the server name and version.
func Test_probe_001(t *testing.T) {
	_, c := newTestServer(t, "probe-server", "2.3.4")
	state, err := c.Probe(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if state.Name == nil || *state.Name != "probe-server" {
		t.Errorf("expected name %q, got %v", "probe-server", state.Name)
	}
	if state.Version == nil || *state.Version != "2.3.4" {
		t.Errorf("expected version %q, got %v", "2.3.4", state.Version)
	}
}

// Test_probe_002: Probe reports tools capability when the server has tools.
func Test_probe_002(t *testing.T) {
	_, c := newTestServer(t, "cap-server", "1.0.0",
		&mock.MockTool{Name_: "tool_x", Description_: "x"},
	)
	state, err := c.Probe(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, cap := range state.Capabilities {
		if cap == schema.CapabilityTools {
			found = true
		}
	}
	if !found {
		t.Errorf("expected tools capability, got %v", state.Capabilities)
	}
}

// Test_probe_003: Probe returns a non-nil ConnectedAt timestamp.
func Test_probe_003(t *testing.T) {
	_, c := newTestServer(t, "ts-server", "1.0.0")
	state, err := c.Probe(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if state.ConnectedAt == nil {
		t.Error("expected non-nil ConnectedAt")
	}
}
