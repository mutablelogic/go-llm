package client_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	mock "github.com/mutablelogic/go-llm/mcp/mock"
	schema "github.com/mutablelogic/go-llm/kernel/schema"
	jsonschema "github.com/mutablelogic/go-server/pkg/jsonschema"
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
	destructive := true
	openWorld := true
	_, c := newTestServer(t, "meta-server", "1.0.0",
		&mock.MockTool{Name_: "my_tool", Description_: "does things", Meta_: llm.ToolMeta{Title: "My Tool", ReadOnlyHint: true, IdempotentHint: true, DestructiveHint: &destructive, OpenWorldHint: &openWorld}},
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
	schema := tool.InputSchema()
	if schema == nil {
		t.Error("expected non-nil InputSchema")
	}
	meta := tool.Meta()
	if meta.Title != "My Tool" {
		t.Errorf("Title: expected %q, got %q", "My Tool", meta.Title)
	}
	if !meta.ReadOnlyHint || !meta.IdempotentHint {
		t.Errorf("expected readonly and idempotent hints, got %+v", meta)
	}
	if meta.DestructiveHint == nil || !*meta.DestructiveHint {
		t.Errorf("expected destructive hint, got %+v", meta.DestructiveHint)
	}
	if meta.OpenWorldHint == nil || !*meta.OpenWorldHint {
		t.Errorf("expected openworld hint, got %+v", meta.OpenWorldHint)
	}
}

// Test_tool_005: CallTool returns ErrBadParameter when required input is missing.
func Test_tool_005(t *testing.T) {
	type args struct {
		URL string `json:"url"`
	}
	_, c := newTestServer(t, "required-server", "1.0.0",
		&mock.MockTool{
			Name_:        "fetch",
			Description_: "fetches a url",
			InputSchema_: jsonschema.MustFor[args](),
			Result_:      "ok",
		},
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

	_, err = tools[0].Run(t.Context(), nil)
	if !errors.Is(err, schema.ErrBadParameter) {
		t.Fatalf("expected ErrBadParameter, got %v", err)
	}
	if err == nil || err.Error() != `bad parameter: input validation failed: validating root: required: missing properties: ["url"]` {
		t.Fatalf("unexpected error: %v", err)
	}
}
