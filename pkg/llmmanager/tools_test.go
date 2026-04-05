package manager

import (
	"context"
	"encoding/json"
	"testing"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	toolkit "github.com/mutablelogic/go-llm/pkg/toolkit"
	pg "github.com/mutablelogic/go-pg"
	jsonschema "github.com/mutablelogic/go-server/pkg/jsonschema"
)

type listToolsMockTool struct {
	name        string
	description string
	schema      *jsonschema.Schema
}

func (t *listToolsMockTool) Name() string                     { return t.name }
func (t *listToolsMockTool) Description() string              { return t.description }
func (t *listToolsMockTool) InputSchema() *jsonschema.Schema  { return t.schema }
func (t *listToolsMockTool) OutputSchema() *jsonschema.Schema { return nil }
func (t *listToolsMockTool) Meta() llm.ToolMeta               { return llm.ToolMeta{} }
func (t *listToolsMockTool) Run(context.Context, json.RawMessage) (any, error) {
	return nil, nil
}

func TestListTools(t *testing.T) {
	tk, err := toolkit.New()
	if err != nil {
		t.Fatal(err)
	}
	if err := tk.AddTool(
		&listToolsMockTool{name: "charlie", description: "C"},
		&listToolsMockTool{name: "alpha", description: "A", schema: jsonschema.MustFor[map[string]any]()},
		&listToolsMockTool{name: "bravo", description: "B"},
	); err != nil {
		t.Fatal(err)
	}

	m := &Manager{Toolkit: tk}
	limit := uint64(2)
	resp, err := m.ListTools(context.Background(), schema.ListToolRequest{
		OffsetLimit: pg.OffsetLimit{Limit: &limit},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Count != 3 {
		t.Fatalf("expected Count=3, got %d", resp.Count)
	}
	if len(resp.Body) != 2 {
		t.Fatalf("expected 2 tools after pagination, got %d", len(resp.Body))
	}
	if resp.Body[0].Name != "builtin.alpha" {
		t.Fatalf("expected first tool %q, got %q", "builtin.alpha", resp.Body[0].Name)
	}
	if resp.Body[1].Name != "builtin.bravo" {
		t.Fatalf("expected second tool %q, got %q", "builtin.bravo", resp.Body[1].Name)
	}
	if string(resp.Body[0].Input) == "" {
		t.Fatal("expected first tool to include input schema")
	}
}
