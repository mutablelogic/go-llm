package server_test

import (
	"context"
	"os"
	"testing"

	// Packages
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	homeassistant "github.com/mutablelogic/go-llm/pkg/homeassistant"
	server "github.com/mutablelogic/go-llm/pkg/mcp/server"
	servertest "github.com/mutablelogic/go-llm/pkg/mcp/server/test"
)

///////////////////////////////////////////////////////////////////////////////
// HELPERS

func haEndpoint(t *testing.T) string {
	t.Helper()
	v := os.Getenv("HA_ENDPOINT")
	if v == "" {
		t.Skip("HA_ENDPOINT not set")
	}
	return v
}

func haToken(t *testing.T) string {
	t.Helper()
	v := os.Getenv("HA_TOKEN")
	if v == "" {
		t.Skip("HA_TOKEN not set")
	}
	return v
}

func haServer(t *testing.T) *server.Server {
	t.Helper()
	tools, err := homeassistant.NewTools(haEndpoint(t), haToken(t))
	if err != nil {
		t.Fatal(err)
	}
	srv, err := server.New("ha-server", "1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	if err := srv.AddTools(tools...); err != nil {
		t.Fatal(err)
	}
	return srv
}

///////////////////////////////////////////////////////////////////////////////
// TESTS

func TestToolList(t *testing.T) {
	_, session := servertest.Connect(t, haServer(t))

	result, err := session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Tools) == 0 {
		t.Fatal("expected at least one tool")
	}
	for _, tool := range result.Tools {
		t.Logf("tool: %s — %s", tool.Name, tool.Description)
	}
}

func TestToolGetStates(t *testing.T) {
	_, session := servertest.Connect(t, haServer(t))

	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{
		Name:      "ha_get_states",
		Arguments: map[string]any{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.IsError {
		t.Fatalf("tool returned error: %v", result.Content)
	}
	if len(result.Content) == 0 {
		t.Fatal("expected at least one content block")
	}
	text, ok := result.Content[0].(*sdkmcp.TextContent)
	if !ok {
		t.Fatalf("expected TextContent, got %T", result.Content[0])
	}
	t.Logf("ha_get_states result: %.200s…", text.Text)
}
