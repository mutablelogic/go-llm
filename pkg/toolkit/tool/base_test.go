package tool

import (
	"testing"

	// Packages
	llm "github.com/mutablelogic/go-llm"
)

///////////////////////////////////////////////////////////////////////////////
// Base

func Test_Base_001_output_schema(t *testing.T) {
	b := Base{}
	s, err := b.OutputSchema()
	if err != nil {
		t.Fatal(err)
	}
	if s != nil {
		t.Fatalf("expected nil, got %v", s)
	}
}

func Test_Base_002_meta(t *testing.T) {
	b := Base{}
	m := b.Meta()
	if m != (llm.ToolMeta{}) {
		t.Fatalf("expected zero ToolMeta, got %+v", m)
	}
}
