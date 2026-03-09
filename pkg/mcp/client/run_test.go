package client_test

import (
	"errors"
	"testing"

	// Packages
	client "github.com/mutablelogic/go-llm/pkg/mcp/client"
)

// Test_run_001: ListTools before Run returns ErrNotConnected.
func Test_run_001(t *testing.T) {
	_, c := newTestServer(t, "test-server", "1.0.0")
	_, err := c.ListTools(t.Context())
	if !errors.Is(err, client.ErrNotConnected) {
		t.Fatalf("expected ErrNotConnected, got %v", err)
	}
}

// Test_run_002: After Run connects, ErrNotConnected is no longer returned.
func Test_run_002(t *testing.T) {
	_, c := newTestServer(t, "test-server", "1.0.0")
	cancel := runClient(t, c)
	defer cancel()
	_, err := c.ListTools(t.Context())
	if errors.Is(err, client.ErrNotConnected) {
		t.Fatal("expected session to be established, got ErrNotConnected")
	}
}
