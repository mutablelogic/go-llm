package manager

import (
	"context"
	"encoding/json"
	"testing"

	// Packages
	jsonschema "github.com/google/jsonschema-go/jsonschema"
	llm "github.com/mutablelogic/go-llm"
	tool "github.com/mutablelogic/go-llm/pkg/tool"
	assert "github.com/stretchr/testify/assert"
)

///////////////////////////////////////////////////////////////////////////////
// MOCK TYPES

type mockTool struct {
	name        string
	description string
	schema      *jsonschema.Schema
}

func (t *mockTool) Name() string                                          { return t.name }
func (t *mockTool) Description() string                                   { return t.description }
func (t *mockTool) Schema() (*jsonschema.Schema, error)                   { return t.schema, nil }
func (t *mockTool) Run(_ context.Context, _ json.RawMessage) (any, error) { return nil, nil }

///////////////////////////////////////////////////////////////////////////////
// TESTS

// Test GetTool returns tool metadata
func Test_tool_001(t *testing.T) {
	assert := assert.New(t)

	tk, err := tool.NewToolkit(&mockTool{name: "my_tool", description: "A test tool"})
	assert.NoError(err)

	m, err := NewManager(WithToolkit(tk))
	assert.NoError(err)

	meta, err := m.GetTool(context.TODO(), "my_tool")
	assert.NoError(err)
	assert.Equal("my_tool", meta.Name)
	assert.Equal("A test tool", meta.Description)
}

// Test GetTool with unknown name returns not found
func Test_tool_002(t *testing.T) {
	assert := assert.New(t)

	m, err := NewManager()
	assert.NoError(err)

	_, err = m.GetTool(context.TODO(), "nonexistent")
	assert.ErrorIs(err, llm.ErrNotFound)
}

// Test GetTool includes JSON schema when present
func Test_tool_003(t *testing.T) {
	assert := assert.New(t)

	s := &jsonschema.Schema{
		Type:        "object",
		Description: "input schema",
	}
	tk, err := tool.NewToolkit(&mockTool{name: "schema_tool", description: "Has schema", schema: s})
	assert.NoError(err)

	m, err := NewManager(WithToolkit(tk))
	assert.NoError(err)

	meta, err := m.GetTool(context.TODO(), "schema_tool")
	assert.NoError(err)
	assert.NotNil(meta.Schema)
	assert.Contains(string(meta.Schema), `"object"`)
}

// Test GetTool omits schema when tool has none
func Test_tool_004(t *testing.T) {
	assert := assert.New(t)

	tk, err := tool.NewToolkit(&mockTool{name: "no_schema", description: "No schema"})
	assert.NoError(err)

	m, err := NewManager(WithToolkit(tk))
	assert.NoError(err)

	meta, err := m.GetTool(context.TODO(), "no_schema")
	assert.NoError(err)
	assert.Nil(meta.Schema)
}
