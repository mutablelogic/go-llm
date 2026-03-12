package tool

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	// Packages
	jsonschema "github.com/google/jsonschema-go/jsonschema"
	llm "github.com/mutablelogic/go-llm"
)

///////////////////////////////////////////////////////////////////////////////
// helpers

type simpleToolForTest struct {
	Base
	name string
}

func (s *simpleToolForTest) Name() string                                          { return s.name }
func (s *simpleToolForTest) Description() string                                   { return "test tool" }
func (s *simpleToolForTest) InputSchema() (*jsonschema.Schema, error)              { return nil, nil }
func (s *simpleToolForTest) Run(_ context.Context, _ json.RawMessage) (any, error) { return nil, nil }

// richToolForTest extends simpleToolForTest with non-nil schemas and meta.
type richToolForTest struct {
	simpleToolForTest
}

type richInput struct {
	Text string `json:"text" jsonschema:"Input text"`
}

type richOutput struct {
	Result string `json:"result" jsonschema:"Output result"`
}

func (r *richToolForTest) InputSchema() (*jsonschema.Schema, error) {
	return jsonschema.For[richInput](nil)
}

func (r *richToolForTest) OutputSchema() (*jsonschema.Schema, error) {
	return jsonschema.For[richOutput](nil)
}

func (r *richToolForTest) Meta() llm.ToolMeta {
	t := true
	return llm.ToolMeta{
		Title:         "Rich Tool",
		ReadOnlyHint:  true,
		OpenWorldHint: &t,
	}
}

// errSchemaToolForTest returns an error from InputSchema to test error propagation.
type errSchemaToolForTest struct {
	simpleToolForTest
}

func (e *errSchemaToolForTest) InputSchema() (*jsonschema.Schema, error) {
	return nil, errors.New("schema generation failed")
}

///////////////////////////////////////////////////////////////////////////////
// WithNamespace

func Test_WithNamespace_001_name(t *testing.T) {
	wrapped := WithNamespace("mynamespace", &simpleToolForTest{name: "mytool"})
	if wrapped.Name() != "mynamespace.mytool" {
		t.Fatalf("expected %q, got %q", "mynamespace.mytool", wrapped.Name())
	}
}

func Test_WithNamespace_002_delegates_other_methods(t *testing.T) {
	inner := &simpleToolForTest{name: "mytool"}
	wrapped := WithNamespace("ns", inner)
	s, err := wrapped.InputSchema()
	if err != nil {
		t.Fatal(err)
	}
	if s != nil {
		t.Fatalf("expected nil schema from inner, got %v", s)
	}
}

func Test_WithNamespace_003_implements_tool_interface(t *testing.T) {
	var _ llm.Tool = WithNamespace("ns", &simpleToolForTest{name: "t"})
}

///////////////////////////////////////////////////////////////////////////////
// Unwrap

func Test_WithNamespace_004_unwrap(t *testing.T) {
	inner := &simpleToolForTest{name: "mytool"}
	wrapped := WithNamespace("ns", inner)
	type unwrapper interface{ Unwrap() llm.Tool }
	u, ok := wrapped.(unwrapper)
	if !ok {
		t.Fatal("wrapped tool does not implement Unwrap()")
	}
	if u.Unwrap() != inner {
		t.Fatal("Unwrap() did not return the original inner tool")
	}
}

func Test_WithNamespace_005_unwrap_double(t *testing.T) {
	// Double-wrapping: Unwrap peels one layer at a time
	inner := &simpleToolForTest{name: "mytool"}
	once := WithNamespace("ns1", inner)
	twice := WithNamespace("ns2", once)
	type unwrapper interface{ Unwrap() llm.Tool }
	u := twice.(unwrapper).Unwrap()
	if u != once {
		t.Fatal("first Unwrap() should return the ns1-wrapped tool")
	}
	if u.(unwrapper).Unwrap() != inner {
		t.Fatal("second Unwrap() should return the original inner tool")
	}
}

///////////////////////////////////////////////////////////////////////////////
// MarshalJSON

func Test_MarshalJSON_001_basic_fields(t *testing.T) {
	// name is the namespaced name, description and annotations are present
	wrapped := WithNamespace("ns", &simpleToolForTest{name: "mytool"})
	data, err := json.Marshal(wrapped)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got["name"] != "ns.mytool" {
		t.Errorf("name: want %q got %q", "ns.mytool", got["name"])
	}
	if got["description"] != "test tool" {
		t.Errorf("description: want %q got %v", "test tool", got["description"])
	}
	if _, ok := got["inputSchema"]; ok {
		t.Error("inputSchema should be absent when InputSchema() returns nil")
	}
	if _, ok := got["outputSchema"]; ok {
		t.Error("outputSchema should be absent when OutputSchema() returns nil")
	}
}

func Test_MarshalJSON_002_with_schemas_and_meta(t *testing.T) {
	// inputSchema, outputSchema, title, and annotations all appear
	inner := &richToolForTest{simpleToolForTest: simpleToolForTest{name: "rich"}}
	wrapped := WithNamespace("svc", inner)
	data, err := json.Marshal(wrapped)
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		Name         string          `json:"name"`
		Title        string          `json:"title"`
		InputSchema  json.RawMessage `json:"inputSchema"`
		OutputSchema json.RawMessage `json:"outputSchema"`
		Annotations  struct {
			ReadOnlyHint  bool  `json:"readOnlyHint"`
			OpenWorldHint *bool `json:"openWorldHint"`
		} `json:"annotations"`
	}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got.Name != "svc.rich" {
		t.Errorf("name: want %q got %q", "svc.rich", got.Name)
	}
	if got.Title != "Rich Tool" {
		t.Errorf("title: want %q got %q", "Rich Tool", got.Title)
	}
	if len(got.InputSchema) == 0 {
		t.Error("inputSchema should be non-empty")
	}
	if len(got.OutputSchema) == 0 {
		t.Error("outputSchema should be non-empty")
	}
	if !got.Annotations.ReadOnlyHint {
		t.Error("annotations.readOnlyHint should be true")
	}
	if got.Annotations.OpenWorldHint == nil || !*got.Annotations.OpenWorldHint {
		t.Error("annotations.openWorldHint should be true")
	}
}

func Test_MarshalJSON_003_input_schema_error_propagated(t *testing.T) {
	inner := &errSchemaToolForTest{simpleToolForTest: simpleToolForTest{name: "errtool"}}
	wrapped := WithNamespace("ns", inner)
	_, err := json.Marshal(wrapped)
	if err == nil {
		t.Fatal("expected an error when InputSchema() fails, got nil")
	}
}
