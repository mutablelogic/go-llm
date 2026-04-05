package server_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"
	"time"

	// Packages
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	homeassistant "github.com/mutablelogic/go-llm/pkg/homeassistant"
	mock "github.com/mutablelogic/go-llm/pkg/mcp/mock"
	server "github.com/mutablelogic/go-llm/pkg/mcp/server"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	jsonschema "github.com/mutablelogic/go-server/pkg/jsonschema"
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
func mockServer(t *testing.T, tools ...*mock.MockTool) *server.Server {
	t.Helper()
	srv, err := server.New("mock-server", "1.0.0")
	if err != nil {
		t.Fatal(err)
	}
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
		&mock.MockTool{Name_: "tool_a", Description_: "Tool A"},
		&mock.MockTool{Name_: "tool_b", Description_: "Tool B"},
	)
	_, session := connect(t, srv)

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
	srv := mockServer(t, &mock.MockTool{
		Name_:        "get_value",
		Description_: "Returns a JSON payload",
		Result_:      payload{Value: 42, Label: "hello"},
	})
	_, session := connect(t, srv)

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
	srv := mockServer(t, &mock.MockTool{
		Name_:        "get_string",
		Description_: "Returns a plain string",
		Result_:      "just text",
	})
	_, session := connect(t, srv)

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
	if tc.Text != `just text` {
		t.Errorf("unexpected text: %q", tc.Text)
	}
}

func TestToolCallMockError(t *testing.T) {
	// Tool error is returned as IsError=true, not a transport error.
	srv := mockServer(t, &mock.MockTool{
		Name_:        "fail_tool",
		Description_: "Always fails",
		RunFn: func(_ context.Context, _ json.RawMessage) (any, error) {
			return nil, errors.New("intentional failure")
		},
	})
	_, session := connect(t, srv)

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
// TESTS — Attachment content types (T4)

func TestToolCallReturnsImage(t *testing.T) {
	srv := mockServer(t, &mock.MockTool{
		Name_:        "get_image",
		Description_: "returns an image",
		Result_:      &schema.Attachment{ContentType: "image/png", Data: []byte{0x89, 0x50, 0x4e, 0x47}},
	})
	_, session := connect(t, srv)

	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{Name: "get_image"})
	if err != nil {
		t.Fatal(err)
	}
	if result.IsError {
		t.Fatalf("unexpected error: %v", result.Content)
	}
	img, ok := result.Content[0].(*sdkmcp.ImageContent)
	if !ok {
		t.Fatalf("expected *sdkmcp.ImageContent, got %T", result.Content[0])
	}
	if img.MIMEType != "image/png" {
		t.Errorf("expected MIME type %q, got %q", "image/png", img.MIMEType)
	}
}

func TestToolCallReturnsAudio(t *testing.T) {
	srv := mockServer(t, &mock.MockTool{
		Name_:        "get_audio",
		Description_: "returns audio",
		Result_:      &schema.Attachment{ContentType: "audio/wav", Data: []byte{0x52, 0x49, 0x46, 0x46}},
	})
	_, session := connect(t, srv)

	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{Name: "get_audio"})
	if err != nil {
		t.Fatal(err)
	}
	if result.IsError {
		t.Fatalf("unexpected error: %v", result.Content)
	}
	audio, ok := result.Content[0].(*sdkmcp.AudioContent)
	if !ok {
		t.Fatalf("expected *sdkmcp.AudioContent, got %T", result.Content[0])
	}
	if audio.MIMEType != "audio/wav" {
		t.Errorf("expected MIME type %q, got %q", "audio/wav", audio.MIMEType)
	}
}

func TestToolCallReturnsBinaryAttachment(t *testing.T) {
	srv := mockServer(t, &mock.MockTool{
		Name_:        "get_doc",
		Description_: "returns a text doc",
		Result_:      &schema.Attachment{ContentType: "text/plain", Data: []byte("hello text")},
	})
	_, session := connect(t, srv)

	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{Name: "get_doc"})
	if err != nil {
		t.Fatal(err)
	}
	if result.IsError {
		t.Fatalf("unexpected error: %v", result.Content)
	}
	tc, ok := result.Content[0].(*sdkmcp.TextContent)
	if !ok {
		t.Fatalf("expected *sdkmcp.TextContent for non-image/audio attachment, got %T", result.Content[0])
	}
	if tc.Text == "" {
		t.Error("expected non-empty text content for attachment")
	}
}

///////////////////////////////////////////////////////////////////////////////
// TESTS — AddTools error paths (T5)

func TestAddToolsBadInputSchema(t *testing.T) {
	srv, err := server.New("test-server", "1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	badSchema, err := jsonschema.FromJSON(json.RawMessage(`{"type":"object"}`))
	if err != nil {
		t.Fatal(err)
	}
	badSchema.Enum = []any{make(chan int)}
	err = srv.AddTools(&mock.MockTool{
		Name_:        "bad_tool",
		Description_: "has bad schema",
		InputSchema_: badSchema,
	})
	if err == nil {
		t.Fatal("expected error for tool with unmarshalable InputSchema, got nil")
	}
}

///////////////////////////////////////////////////////////////////////////////
// TESTS — Session context injection (T6)

func TestToolRunSessionContext(t *testing.T) {
	type capture struct {
		hasLogger     bool
		hasClientInfo bool
		id            string
	}
	ch := make(chan capture, 1)
	srv := mockServer(t, &mock.MockTool{
		Name_:        "ctx_tool",
		Description_: "inspects session context",
		RunFn: func(ctx context.Context, _ json.RawMessage) (any, error) {
			sess := server.SessionFromContext(ctx)
			ch <- capture{
				hasLogger:     sess.Logger() != nil,
				hasClientInfo: sess.ClientInfo() != nil,
				id:            sess.ID(),
			}
			return "ok", nil
		},
	})
	_, session := connect(t, srv)

	_, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{Name: "ctx_tool"})
	if err != nil {
		t.Fatal(err)
	}
	select {
	case res := <-ch:
		if !res.hasLogger {
			t.Error("expected non-nil Logger in session context")
		}
		if !res.hasClientInfo {
			t.Error("expected non-nil ClientInfo in session context")
		}
		if res.id == "" {
			t.Error("expected non-empty session ID")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("tool was not called within 2s")
	}
}

///////////////////////////////////////////////////////////////////////////////
// TESTS — Panic recovery (B3)

func TestToolRunPanic(t *testing.T) {
	srv := mockServer(t, &mock.MockTool{
		Name_:        "panic_tool",
		Description_: "always panics",
		RunFn: func(_ context.Context, _ json.RawMessage) (any, error) {
			panic("intentional test panic")
		},
	})
	_, session := connect(t, srv)

	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{Name: "panic_tool"})
	if err != nil {
		t.Fatal(err)
	}
	if !result.IsError {
		t.Fatal("expected IsError=true for panicking tool; server session should survive")
	}
}

///////////////////////////////////////////////////////////////////////////////
// TESTS — Home Assistant (require env vars)

func TestToolList(t *testing.T) {
	_, session := connect(t, haServer(t))

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
	_, session := connect(t, haServer(t))

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
