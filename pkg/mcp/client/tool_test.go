package client_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	// Packages
	mock "github.com/mutablelogic/go-llm/pkg/mcp/mock"
)

// Test_tool_001: CallTool returns a JSON object as json.RawMessage.
func Test_tool_001(t *testing.T) {
	type payload struct {
		Value int    `json:"value"`
		Label string `json:"label"`
	}
	_, c := newTestServer(t, "call-server", "1.0.0",
		&mock.MockTool{
			Name_:        "get_value",
			Description_: "returns a struct",
			Result_:      payload{Value: 42, Label: "hello"},
		},
	)
	cancel := runClient(t, c)
	defer cancel()

	got, err := c.CallTool(t.Context(), "get_value", nil)
	if err != nil {
		t.Fatal(err)
	}
	raw, ok := got.(json.RawMessage)
	if !ok {
		t.Fatalf("expected json.RawMessage, got %T", got)
	}
	var res payload
	if err := json.Unmarshal(raw, &res); err != nil {
		t.Fatal(err)
	}
	if res.Value != 42 || res.Label != "hello" {
		t.Errorf("unexpected result: %+v", res)
	}
}

// Test_tool_002: CallTool returns a plain string as string (no JSON-quoting).
func Test_tool_002(t *testing.T) {
	_, c := newTestServer(t, "str-server", "1.0.0",
		&mock.MockTool{
			Name_:        "greet",
			Description_: "returns a greeting",
			Result_:      "hello world",
		},
	)
	cancel := runClient(t, c)
	defer cancel()

	got, err := c.CallTool(t.Context(), "greet", nil)
	if err != nil {
		t.Fatal(err)
	}
	s, ok := got.(string)
	if !ok {
		t.Fatalf("expected string, got %T", got)
	}
	if s != "hello world" {
		t.Errorf("expected %q, got %q", "hello world", s)
	}
}

// Test_tool_003: CallTool propagates tool errors as Go errors.
func Test_tool_003(t *testing.T) {
	_, c := newTestServer(t, "err-server", "1.0.0",
		&mock.MockTool{
			Name_: "fail_tool", Description_: "always errors",
			RunFn: func(_ context.Context, _ json.RawMessage) (any, error) {
				return nil, errors.New("something went wrong")
			},
		},
	)
	cancel := runClient(t, c)
	defer cancel()

	_, err := c.CallTool(t.Context(), "fail_tool", nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err.Error() != "something went wrong" {
		t.Errorf("unexpected error: %q", err.Error())
	}
}

// Test_tool_004: Tools returned by ListTools implement llm.Tool correctly.
func Test_tool_004(t *testing.T) {
	_, c := newTestServer(t, "meta-server", "1.0.0",
		&mock.MockTool{Name_: "my_tool", Description_: "does things"},
	)
	cancel := runClient(t, c)
	defer cancel()

	tools, err := c.ListTools(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if len(tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(tools))
	}
	tool := tools[0]
	if tool.Name() != "my_tool" {
		t.Errorf("Name: expected %q, got %q", "my_tool", tool.Name())
	}
	if tool.Description() != "does things" {
		t.Errorf("Description: expected %q, got %q", "does things", tool.Description())
	}
	schema, err := tool.InputSchema()
	if err != nil {
		t.Fatalf("InputSchema error: %v", err)
	}
	if schema == nil {
		t.Error("expected non-nil InputSchema")
	}
}
