package tool

import (
	"context"
	"encoding/json"
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
