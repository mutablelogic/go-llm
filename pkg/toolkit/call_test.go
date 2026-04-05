package toolkit

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	resource "github.com/mutablelogic/go-llm/pkg/toolkit/resource"
	jsonschema "github.com/mutablelogic/go-server/pkg/jsonschema"
	gootel "go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	tracenoop "go.opentelemetry.io/otel/trace/noop"
)

///////////////////////////////////////////////////////////////////////////////
// MOCK TYPES for call tests

// callableTool controls what Run returns.
type callableTool struct {
	name         string
	result       any
	err          error
	inputSchema  *jsonschema.Schema
	outputSchema *jsonschema.Schema
}

func (m *callableTool) Name() string                                          { return m.name }
func (m *callableTool) Description() string                                   { return "callable tool " + m.name }
func (m *callableTool) InputSchema() *jsonschema.Schema                       { return m.inputSchema }
func (m *callableTool) OutputSchema() *jsonschema.Schema                      { return m.outputSchema }
func (m *callableTool) Meta() llm.ToolMeta                                    { return llm.ToolMeta{} }
func (m *callableTool) Run(_ context.Context, _ json.RawMessage) (any, error) { return m.result, m.err }

///////////////////////////////////////////////////////////////////////////////
// Call — bad key type

func Test_Call_001_bad_key_type(t *testing.T) {
	tk, _ := New()
	_, err := tk.Call(context.Background(), 42)
	if !errors.Is(err, schema.ErrBadParameter) {
		t.Fatalf("expected ErrBadParameter, got %v", err)
	}
}

///////////////////////////////////////////////////////////////////////////////
// Call — string key

func Test_Call_002_string_key_not_found(t *testing.T) {
	tk, _ := New()
	_, err := tk.Call(context.Background(), "nonexistent")
	if !errors.Is(err, schema.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func Test_Call_003_string_key_tool(t *testing.T) {
	tk, _ := New()
	tool := &callableTool{name: "greet", result: "hello"}
	_ = tk.AddTool(tool)
	res, err := tk.Call(context.Background(), "greet")
	if err != nil {
		t.Fatal(err)
	}
	if res == nil {
		t.Fatal("expected non-nil resource")
	}
}

///////////////////////////////////////////////////////////////////////////////
// Call — direct llm.Tool / llm.Prompt

func Test_Call_004_direct_tool(t *testing.T) {
	tk, _ := New()
	tool := &callableTool{name: "greet", result: "hello"}
	res, err := tk.Call(context.Background(), tool)
	if err != nil {
		t.Fatal(err)
	}
	if res == nil {
		t.Fatal("expected non-nil resource")
	}
}

func Test_Call_005_direct_prompt_no_delegate(t *testing.T) {
	tk, _ := New()
	_, err := tk.Call(context.Background(), &mockPrompt{name: "summarize"})
	if !errors.Is(err, schema.ErrNotImplemented) {
		t.Fatalf("expected ErrNotImplemented, got %v", err)
	}
}

func Test_Call_006_direct_prompt_with_delegate(t *testing.T) {
	d := &mockDelegate{}
	tk, _ := New(WithDelegate(d))
	p := &mockPrompt{name: "summarize"}
	// mockDelegate.Call returns nil, nil — that is a valid (nil resource) result.
	_, err := tk.Call(context.Background(), p)
	if err != nil {
		t.Fatal(err)
	}
}

///////////////////////////////////////////////////////////////////////////////
// callTool — return type variants

func Test_Call_007_tool_returns_nil_no_output_schema(t *testing.T) {
	tk, _ := New()
	tool := &callableTool{name: "noop", result: nil}
	res, err := tk.Call(context.Background(), tool)
	if err != nil {
		t.Fatal(err)
	}
	if res != nil {
		t.Fatalf("expected nil resource, got %v", res)
	}
}

func Test_Call_008_tool_returns_string(t *testing.T) {
	tk, _ := New()
	tool := &callableTool{name: "greet", result: "hello world"}
	res, err := tk.Call(context.Background(), tool)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := res.Read(context.Background())
	if string(data) != "hello world" {
		t.Fatalf("expected %q, got %q", "hello world", string(data))
	}
}

func Test_Call_009_tool_returns_bytes(t *testing.T) {
	tk, _ := New()
	tool := &callableTool{name: "greet", result: []byte("hello bytes")}
	res, err := tk.Call(context.Background(), tool)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := res.Read(context.Background())
	if string(data) != "hello bytes" {
		t.Fatalf("expected %q, got %q", "hello bytes", string(data))
	}
}

func Test_Call_010_tool_returns_raw_json(t *testing.T) {
	tk, _ := New()
	tool := &callableTool{name: "greet", result: json.RawMessage(`{"ok":true}`)}
	res, err := tk.Call(context.Background(), tool)
	if err != nil {
		t.Fatal(err)
	}
	if res == nil {
		t.Fatal("expected non-nil resource")
	}
}

func Test_Call_011_tool_returns_resource(t *testing.T) {
	tk, _ := New()
	inner, _ := resource.Text("inner", "content")
	tool := &callableTool{name: "greet", result: inner}
	res, err := tk.Call(context.Background(), tool)
	if err != nil {
		t.Fatal(err)
	}
	if res == nil {
		t.Fatal("expected non-nil resource")
	}
}

func Test_Call_012_tool_returns_bad_type(t *testing.T) {
	tk, _ := New()
	tool := &callableTool{name: "greet", result: 12345}
	_, err := tk.Call(context.Background(), tool)
	if !errors.Is(err, schema.ErrBadParameter) {
		t.Fatalf("expected ErrBadParameter, got %v", err)
	}
}

func Test_Call_013_tool_propagates_run_error(t *testing.T) {
	tk, _ := New()
	tool := &callableTool{name: "fail", err: errors.New("boom")}
	_, err := tk.Call(context.Background(), tool)
	if err == nil || err.Error() != "boom" {
		t.Fatalf("expected 'boom', got %v", err)
	}
}

///////////////////////////////////////////////////////////////////////////////
// callTool — resource input validation

func Test_Call_014_too_many_resources(t *testing.T) {
	tk, _ := New()
	tool := &callableTool{name: "greet"}
	r1, _ := resource.Text("a", "x")
	r2, _ := resource.Text("b", "y")
	_, err := tk.Call(context.Background(), tool, r1, r2)
	if !errors.Is(err, schema.ErrBadParameter) {
		t.Fatalf("expected ErrBadParameter, got %v", err)
	}
}

func Test_Call_015_nil_resource(t *testing.T) {
	tk, _ := New()
	tool := &callableTool{name: "greet"}
	_, err := tk.Call(context.Background(), tool, nil)
	if !errors.Is(err, schema.ErrBadParameter) {
		t.Fatalf("expected ErrBadParameter, got %v", err)
	}
}

func Test_Call_016_non_json_resource(t *testing.T) {
	tk, _ := New()
	tool := &callableTool{name: "greet"}
	r, _ := resource.Text("input", "not json")
	_, err := tk.Call(context.Background(), tool, r)
	if !errors.Is(err, schema.ErrBadParameter) {
		t.Fatalf("expected ErrBadParameter, got %v", err)
	}
}

func Test_Call_017_json_resource_input(t *testing.T) {
	tk, _ := New()
	tool := &callableTool{name: "greet", result: "ok"}
	r, _ := resource.JSON("input", map[string]string{"name": "world"})
	res, err := tk.Call(context.Background(), tool, r)
	if err != nil {
		t.Fatal(err)
	}
	if res == nil {
		t.Fatal("expected non-nil resource")
	}
}

///////////////////////////////////////////////////////////////////////////////
// callTool — input schema validation

func Test_Call_018_input_schema_validates_valid_input(t *testing.T) {
	type args struct {
		Name string `json:"name"`
	}
	s := jsonschema.MustFor[args]()
	tk, _ := New()
	tool := &callableTool{name: "t", result: "ok", inputSchema: s}
	r, _ := resource.JSON("input", args{Name: "world"})
	res, err := tk.Call(context.Background(), tool, r)
	if err != nil {
		t.Fatal(err)
	}
	if res == nil {
		t.Fatal("expected non-nil resource")
	}
}
func Test_Call_019_input_schema_rejects_invalid_input(t *testing.T) {
	type args struct {
		Name string `json:"name"`
	}
	s := jsonschema.MustFor[args]()
	tk, _ := New()
	tool := &callableTool{name: "t", result: "ok", inputSchema: s}
	// Pass a number instead of an object — should fail schema validation.
	r, _ := resource.JSON("input", 42)
	_, err := tk.Call(context.Background(), tool, r)
	if !errors.Is(err, schema.ErrBadParameter) {
		t.Fatalf("expected ErrBadParameter for invalid input, got %v", err)
	}
}

///////////////////////////////////////////////////////////////////////////////
// callTool — nil result + output schema

func Test_Call_020_nil_result_with_output_schema_errors(t *testing.T) {
	type out struct {
		Value string `json:"value"`
	}
	s := jsonschema.MustFor[out]()
	tk, _ := New()
	tool := &callableTool{name: "t", result: nil, outputSchema: s}
	_, err := tk.Call(context.Background(), tool)
	if !errors.Is(err, schema.ErrBadParameter) {
		t.Fatalf("expected ErrBadParameter when nil result + output schema, got %v", err)
	}
}

///////////////////////////////////////////////////////////////////////////////
// callTool — output schema validation

func Test_Call_021_output_schema_validates_json_output(t *testing.T) {
	type out struct {
		Value string `json:"value"`
	}
	s := jsonschema.MustFor[out]()
	tk, _ := New()
	tool := &callableTool{name: "t", result: json.RawMessage(`{"value":"ok"}`), outputSchema: s}
	res, err := tk.Call(context.Background(), tool)
	if err != nil {
		t.Fatal(err)
	}
	if res == nil {
		t.Fatal("expected non-nil resource")
	}
}

func Test_Call_022_output_schema_rejects_invalid_json_output(t *testing.T) {
	type out struct {
		Value string `json:"value"`
	}
	s := jsonschema.MustFor[out]()
	tk, _ := New()
	// Return a number instead of an object.
	tool := &callableTool{name: "t", result: json.RawMessage(`42`), outputSchema: s}
	_, err := tk.Call(context.Background(), tool)
	if !errors.Is(err, schema.ErrBadParameter) {
		t.Fatalf("expected ErrBadParameter for invalid output, got %v", err)
	}
}

func Test_Call_023_output_schema_with_non_json_output_errors(t *testing.T) {
	type out struct {
		Value string `json:"value"`
	}
	s := jsonschema.MustFor[out]()
	tk, _ := New()
	// String output is not JSON content-type, so validation should fail.
	tool := &callableTool{name: "t", result: "plain string", outputSchema: s}
	_, err := tk.Call(context.Background(), tool)
	if !errors.Is(err, schema.ErrBadParameter) {
		t.Fatalf("expected ErrBadParameter for non-JSON output with schema, got %v", err)
	}
}

///////////////////////////////////////////////////////////////////////////////
// callTool — otel tracer propagates traceparent into session meta

func Test_Call_024_with_real_tracer_injects_traceparent(t *testing.T) {
	// Register the W3C TraceContext propagator globally so traceparent is injected.
	oldPropagator := gootel.GetTextMapPropagator()
	gootel.SetTextMapPropagator(propagation.TraceContext{})
	t.Cleanup(func() { gootel.SetTextMapPropagator(oldPropagator) })

	// Use the SDK tracer provider so a real span is started.
	tp := sdktrace.NewTracerProvider()
	defer tp.Shutdown(context.Background())
	tracer := tp.Tracer("test")

	var capturedMeta map[string]any
	captureTool := &captureMetaTool{name: "t", result: "ok", meta: &capturedMeta}

	tk, _ := New(WithTracer(tracer))
	res, err := tk.Call(context.Background(), captureTool)
	if err != nil {
		t.Fatal(err)
	}
	if res == nil {
		t.Fatal("expected non-nil resource")
	}
	if capturedMeta["traceparent"] == "" || capturedMeta["traceparent"] == nil {
		t.Fatalf("expected traceparent in session meta, got %v", capturedMeta)
	}
}

func Test_Call_028_with_noop_tracer_no_traceparent(t *testing.T) {
	// Noop tracer produces an invalid span — carrier stays empty.
	tracer := tracenoop.NewTracerProvider().Tracer("test")
	var capturedMeta map[string]any
	captureTool := &captureMetaTool{name: "t", result: "ok", meta: &capturedMeta}

	tk, _ := New(WithTracer(tracer))
	_, err := tk.Call(context.Background(), captureTool)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := capturedMeta["traceparent"]; ok {
		t.Fatalf("expected no traceparent from noop tracer, got %v", capturedMeta["traceparent"])
	}
}

// captureMetaTool captures the session meta from its context on Run.
type captureMetaTool struct {
	name   string
	result any
	meta   *map[string]any
}

func (m *captureMetaTool) Name() string                     { return m.name }
func (m *captureMetaTool) Description() string              { return "capture meta tool" }
func (m *captureMetaTool) InputSchema() *jsonschema.Schema  { return nil }
func (m *captureMetaTool) OutputSchema() *jsonschema.Schema { return nil }
func (m *captureMetaTool) Meta() llm.ToolMeta               { return llm.ToolMeta{} }
func (m *captureMetaTool) Run(ctx context.Context, _ json.RawMessage) (any, error) {
	sess := SessionFromContext(ctx)
	if m.meta != nil {
		*m.meta = sess.Meta()
	}
	return m.result, nil
}
