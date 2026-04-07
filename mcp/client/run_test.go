package client_test

import (
	"errors"
	"testing"

	// Packages

	schema "github.com/mutablelogic/go-llm/kernel/schema"
)

// Test_run_001: ListTools before Run returns ErrServiceUnavailable.
func Test_run_001(t *testing.T) {
	_, c := newTestServer(t, "test-server", "1.0.0")
	_, err := c.ListTools(t.Context())
	if !errors.Is(err, schema.ErrServiceUnavailable) {
		t.Fatalf("expected ErrServiceUnavailable, got %v", err)
	}
}

// Test_run_002: After Run connects, ErrServiceUnavailable is no longer returned.
func Test_run_002(t *testing.T) {
	_, c := newTestServer(t, "test-server", "1.0.0")
	cancel := runClient(t, c)
	defer cancel()
	_, err := c.ListTools(t.Context())
	if errors.Is(err, schema.ErrServiceUnavailable) {
		t.Fatal("expected session to be established, got ErrServiceUnavailable")
	}
}
