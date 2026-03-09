// Package servertest provides test helpers for the pkg/mcp/server package,
// including a MockTool and a Connect helper for obtaining an SDK ClientSession
// against a running Server without needing real credentials.
package servertest

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"

	// Packages
	jsonschema "github.com/google/jsonschema-go/jsonschema"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	llm "github.com/mutablelogic/go-llm"
	server "github.com/mutablelogic/go-llm/pkg/mcp/server"
	tool "github.com/mutablelogic/go-llm/pkg/tool"
)

// MockTool is a configurable implementation of tool.Tool for use in tests.
// Set Name_, Description_, and Result_ before registering it on a server.
// RunFn, if set, overrides Result_ and is called with the raw JSON input.
type MockTool struct {
	tool.DefaultTool
	Name_        string
	Description_ string
	InputSchema_ *jsonschema.Schema
	Result_      any
	RunFn        func(ctx context.Context, input json.RawMessage) (any, error)
}

var _ llm.Tool = (*MockTool)(nil)

func (m *MockTool) Name() string        { return m.Name_ }
func (m *MockTool) Description() string { return m.Description_ }

func (m *MockTool) InputSchema() (*jsonschema.Schema, error) {
	if m.InputSchema_ != nil {
		return m.InputSchema_, nil
	}
	return &jsonschema.Schema{Type: "object"}, nil
}

func (m *MockTool) Run(ctx context.Context, input json.RawMessage) (any, error) {
	if m.RunFn != nil {
		return m.RunFn(ctx, input)
	}
	return m.Result_, nil
}

// Connect starts an httptest.Server wrapping srv and returns a connected
// *sdkmcp.ClientSession. Cleanup is registered with t.Cleanup automatically.
func Connect(t *testing.T, srv *server.Server) (*httptest.Server, *sdkmcp.ClientSession) {
	t.Helper()
	ts := httptest.NewServer(srv.Handler())
	mc := sdkmcp.NewClient(&sdkmcp.Implementation{Name: "test-client", Version: "1.0.0"}, nil)
	session, err := mc.Connect(context.Background(), &sdkmcp.StreamableClientTransport{Endpoint: ts.URL}, nil)
	if err != nil {
		ts.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		session.Close()
		ts.Close()
	})
	return ts, session
}
