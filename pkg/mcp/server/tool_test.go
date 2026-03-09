package server_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"

	// Packages
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	homeassistant "github.com/mutablelogic/go-llm/pkg/homeassistant"
	server "github.com/mutablelogic/go-llm/pkg/mcp/server"
	servertest "github.com/mutablelogic/go-llm/pkg/mcp/server/test"
	types "github.com/mutablelogic/go-server/pkg/types"
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

// mockServer creates a Server with the given MockTools registered.
func mockServer(t *testing.T, tools ...*servertest.MockTool) *server.Server {
	t.Helper()
	srv, err := server.New("mock-server", "1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	llmTools := make([]interface{ Name() string }, 0, len(tools))
	_ = llmTools
	iTools := make([]interface{}, 0, len(tools))
	_ = iTools
	for _, mt := range tools {
		if err := srv.AddTools(mt); err != nil {
			t.Fatal(err)
		}
	}
	return srv
}

///////////////////////////////////////////////////////////////////////////////
// TESTS — MockTool (no env vars required)

func TestToolListMock(t *testing.T) {
	srv := mockServer(t,
		&servertest.MockTool{Name_: "tool_a", Description_: "Tool A"},
		&servertest.MockTool{Name_: "tool_b", Description_: "Tool B"},
	)
	_, session := servertest.Connect(t, srv)

	result, err := session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Tools) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(result.Tools))
	}
}

func TestToolCallMockReturnsJSON(t *testing.T) {
	// Tool returns a struct; client should receive it back as JSON (not a double-encoded string).
	type payload struct {
		Value int    `json:"value"`
		Label string `json:"label"`
	}
	srv := mockServer(t, &servertest.MockTool{
		Name_:        "get_value",
		Description_: "Returns a JSON payload",
		Result_:      payload{Value: 42, Label: "hello"},
	})
	_, session := servertest.Connect(t, srv)

	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{
		Name: "get_value",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.IsError {
		t.Fatalf("tool returned error: %v", result.Content)
	}
	if len(result.Content) != 1 {
		t.Fatalf("expected 1 content block, got %d", len(result.Content))
	}
	tc, ok := result.Content[0].(*sdkmcp.TextContent)
	if !ok {
		t.Fatalf("expected *sdkmcp.TextContent, got %T", result.Content[0])
	}
	// Should be tagged with application/json so clients can decode it.
	if tc.Meta[types.ContentTypeHeader] != types.ContentTypeJSON {
		t.Errorf("expected Content-Type %q, got %q", types.ContentTypeJSON, tc.Meta[types.ContentTypeHeader])
	}
	var got payload
	if err := json.Unmarshal([]byte(tc.Text), &got); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if got.Value != 42 || got.Label != "hello" {
		t.Errorf("unexpected payload: %+v", got)
	}
}

func TestToolCallMockReturnsString(t *testing.T) {
	// Tool returns a plain string; should not be tagged as JSON.
	srv := mockServer(t, &servertest.MockTool{
		Name_:        "get_string",
		Description_: "Returns a plain string",
		Result_:      "just text",
	})
	_, session := servertest.Connect(t, srv)

	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{
		Name: "get_string",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.IsError {
		t.Fatalf("tool returned error: %v", result.Content)
	}
	tc, ok := result.Content[0].(*sdkmcp.TextContent)
	if !ok {
		t.Fatalf("expected *sdkmcp.TextContent, got %T", result.Content[0])
	}
	if tc.Meta[types.ContentTypeHeader] == types.ContentTypeJSON {
		t.Errorf("plain string should not be tagged as application/json")
	}
	if tc.Text != `"just text"` {
		t.Errorf("unexpected text: %q", tc.Text)
	}
}

func TestToolCallMockError(t *testing.T) {
	// Tool error is returned as IsError=true, not a transport error.
	srv := mockServer(t, &servertest.MockTool{
		Name_:        "fail_tool",
		Description_: "Always fails",
		RunFn: func(_ context.Context, _ json.RawMessage) (any, error) {
			return nil, errors.New("intentional failure")
		},
	})
	_, session := servertest.Connect(t, srv)

	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{
		Name: "fail_tool",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.IsError {
		t.Fatal("expected IsError=true")
	}
}

///////////////////////////////////////////////////////////////////////////////
// TESTS — Home Assistant (require env vars)

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
