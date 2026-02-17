package manager

import (
	"context"
	"encoding/json"
	"testing"

	// Packages
	jsonschema "github.com/google/jsonschema-go/jsonschema"
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	tool "github.com/mutablelogic/go-llm/pkg/tool"
	assert "github.com/stretchr/testify/assert"
)

///////////////////////////////////////////////////////////////////////////////
// MOCK TYPES

type mockTool struct {
	name        string
	description string
	schema      *jsonschema.Schema
	runFn       func(context.Context, json.RawMessage) (any, error)
}

func (t *mockTool) Name() string                        { return t.name }
func (t *mockTool) Description() string                 { return t.description }
func (t *mockTool) Schema() (*jsonschema.Schema, error) { return t.schema, nil }
func (t *mockTool) Run(ctx context.Context, input json.RawMessage) (any, error) {
	if t.runFn != nil {
		return t.runFn(ctx, input)
	}
	return nil, nil
}

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
	assert.NotNil(meta.Input)
	assert.Contains(string(meta.Input), `"object"`)
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
	assert.Nil(meta.Input)
}

// Test CallTool executes a tool and returns the result
func Test_tool_005(t *testing.T) {
	assert := assert.New(t)

	tk, err := tool.NewToolkit(&mockTool{
		name:        "echo_tool",
		description: "Echoes input",
		runFn: func(_ context.Context, input json.RawMessage) (any, error) {
			return map[string]string{"echoed": string(input)}, nil
		},
	})
	assert.NoError(err)

	m, err := NewManager(WithToolkit(tk))
	assert.NoError(err)

	resp, err := m.CallTool(context.TODO(), "echo_tool", json.RawMessage(`"hello"`))
	assert.NoError(err)
	assert.Equal("echo_tool", resp.Tool)
	assert.Contains(string(resp.Result), `"echoed"`)
}

// Test CallTool with unknown tool returns not found
func Test_tool_006(t *testing.T) {
	assert := assert.New(t)

	m, err := NewManager()
	assert.NoError(err)

	_, err = m.CallTool(context.TODO(), "nonexistent", nil)
	assert.ErrorIs(err, llm.ErrNotFound)
}

// Test CallTool with nil input
func Test_tool_007(t *testing.T) {
	assert := assert.New(t)

	tk, err := tool.NewToolkit(&mockTool{
		name: "nil_input",
		runFn: func(_ context.Context, input json.RawMessage) (any, error) {
			return "ok", nil
		},
	})
	assert.NoError(err)

	m, err := NewManager(WithToolkit(tk))
	assert.NoError(err)

	resp, err := m.CallTool(context.TODO(), "nil_input", nil)
	assert.NoError(err)
	assert.Equal(`"ok"`, string(resp.Result))
}

// Test CallTool propagates tool execution errors
func Test_tool_008(t *testing.T) {
	assert := assert.New(t)

	tk, err := tool.NewToolkit(&mockTool{
		name: "fail_tool",
		runFn: func(_ context.Context, _ json.RawMessage) (any, error) {
			return nil, llm.ErrBadParameter.With("bad input")
		},
	})
	assert.NoError(err)

	m, err := NewManager(WithToolkit(tk))
	assert.NoError(err)

	_, err = m.CallTool(context.TODO(), "fail_tool", json.RawMessage(`{}`))
	assert.ErrorIs(err, llm.ErrBadParameter)
}

// Test ListTools returns all tools sorted by name
func Test_tool_009(t *testing.T) {
	assert := assert.New(t)

	tk, err := tool.NewToolkit(
		&mockTool{name: "charlie", description: "C"},
		&mockTool{name: "alpha", description: "A"},
		&mockTool{name: "bravo", description: "B"},
	)
	assert.NoError(err)

	m, err := NewManager(WithToolkit(tk))
	assert.NoError(err)

	resp, err := m.ListTools(context.TODO(), schema.ListToolRequest{})
	assert.NoError(err)
	assert.Equal(uint(3), resp.Count)
	assert.Equal("alpha", resp.Body[0].Name)
	assert.Equal("bravo", resp.Body[1].Name)
	assert.Equal("charlie", resp.Body[2].Name)
}

// Test ListTools with limit
func Test_tool_010(t *testing.T) {
	assert := assert.New(t)

	tk, err := tool.NewToolkit(
		&mockTool{name: "alpha"},
		&mockTool{name: "bravo"},
		&mockTool{name: "charlie"},
	)
	assert.NoError(err)

	m, err := NewManager(WithToolkit(tk))
	assert.NoError(err)

	limit := uint(2)
	resp, err := m.ListTools(context.TODO(), schema.ListToolRequest{Limit: &limit})
	assert.NoError(err)
	assert.Equal(uint(3), resp.Count)
	assert.Len(resp.Body, 2)
	assert.Equal("alpha", resp.Body[0].Name)
	assert.Equal("bravo", resp.Body[1].Name)
}

// Test ListTools with offset
func Test_tool_011(t *testing.T) {
	assert := assert.New(t)

	tk, err := tool.NewToolkit(
		&mockTool{name: "alpha"},
		&mockTool{name: "bravo"},
		&mockTool{name: "charlie"},
	)
	assert.NoError(err)

	m, err := NewManager(WithToolkit(tk))
	assert.NoError(err)

	resp, err := m.ListTools(context.TODO(), schema.ListToolRequest{Offset: 1})
	assert.NoError(err)
	assert.Equal(uint(3), resp.Count)
	assert.Len(resp.Body, 2)
	assert.Equal("bravo", resp.Body[0].Name)
}

// Test ListTools with limit and offset
func Test_tool_012(t *testing.T) {
	assert := assert.New(t)

	tk, err := tool.NewToolkit(
		&mockTool{name: "alpha"},
		&mockTool{name: "bravo"},
		&mockTool{name: "charlie"},
	)
	assert.NoError(err)

	m, err := NewManager(WithToolkit(tk))
	assert.NoError(err)

	limit := uint(1)
	resp, err := m.ListTools(context.TODO(), schema.ListToolRequest{Limit: &limit, Offset: 1})
	assert.NoError(err)
	assert.Equal(uint(3), resp.Count)
	assert.Len(resp.Body, 1)
	assert.Equal("bravo", resp.Body[0].Name)
}

// Test ListTools empty
func Test_tool_013(t *testing.T) {
	assert := assert.New(t)

	m, err := NewManager()
	assert.NoError(err)

	resp, err := m.ListTools(context.TODO(), schema.ListToolRequest{})
	assert.NoError(err)
	assert.Equal(uint(0), resp.Count)
	assert.Empty(resp.Body)
}

// Test ListTools offset beyond total
func Test_tool_014(t *testing.T) {
	assert := assert.New(t)

	tk, err := tool.NewToolkit(&mockTool{name: "alpha"})
	assert.NoError(err)

	m, err := NewManager(WithToolkit(tk))
	assert.NoError(err)

	resp, err := m.ListTools(context.TODO(), schema.ListToolRequest{Offset: 10})
	assert.NoError(err)
	assert.Equal(uint(1), resp.Count)
	assert.Empty(resp.Body)
}

// Test ListTools includes input schema
func Test_tool_015(t *testing.T) {
	assert := assert.New(t)

	s := &jsonschema.Schema{Type: "string"}
	tk, err := tool.NewToolkit(&mockTool{name: "typed_tool", schema: s})
	assert.NoError(err)

	m, err := NewManager(WithToolkit(tk))
	assert.NoError(err)

	resp, err := m.ListTools(context.TODO(), schema.ListToolRequest{})
	assert.NoError(err)
	assert.Len(resp.Body, 1)
	assert.NotNil(resp.Body[0].Input)
	assert.Contains(string(resp.Body[0].Input), `"string"`)
}

// Test CallTool result marshalling
func Test_tool_016(t *testing.T) {
	assert := assert.New(t)

	type weatherResult struct {
		Temp float64 `json:"temp"`
		Unit string  `json:"unit"`
	}

	tk, err := tool.NewToolkit(&mockTool{
		name: "get_weather",
		runFn: func(_ context.Context, _ json.RawMessage) (any, error) {
			return weatherResult{Temp: 22.5, Unit: "celsius"}, nil
		},
	})
	assert.NoError(err)

	m, err := NewManager(WithToolkit(tk))
	assert.NoError(err)

	resp, err := m.CallTool(context.TODO(), "get_weather", nil)
	assert.NoError(err)
	assert.Equal("get_weather", resp.Tool)

	var result weatherResult
	assert.NoError(json.Unmarshal(resp.Result, &result))
	assert.Equal(22.5, result.Temp)
	assert.Equal("celsius", result.Unit)
}
