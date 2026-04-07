package server_test

import (
	"context"
	"net/http/httptest"
	"testing"

	// Packages
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	server "github.com/mutablelogic/go-llm/mcp/server"
)

// connect starts an httptest.Server wrapping srv and returns a connected
// *sdkmcp.ClientSession. Cleanup is registered with t.Cleanup automatically.
func connect(t *testing.T, srv *server.Server) (*httptest.Server, *sdkmcp.ClientSession) {
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
