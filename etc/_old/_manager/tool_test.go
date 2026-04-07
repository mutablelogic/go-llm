package manager

import (
	"context"
	"encoding/json"
	"testing"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/kernel/schema"
	tool "github.com/mutablelogic/go-llm/pkg/tool"
	jsonschema "github.com/mutablelogic/go-server/pkg/jsonschema"
	assert "github.com/stretchr/testify/assert"
)

///////////////////////////////////////////////////////////////////////////////
// MOCK TYPES

type mockTool struct {
	name        string
	description string
	input       *jsonschema.Schema
	output      *jsonschema.Schema
	meta        llm.ToolMeta
	runFn       func(context.Context, json.RawMessage) (any, error)
}

func (t *mockTool) Name() string                     { return t.name }
func (t *mockTool) Description() string              { return t.description }
func (t *mockTool) InputSchema() *jsonschema.Schema  { return t.input }
func (t *mockTool) OutputSchema() *jsonschema.Schema { return t.output }
func (t *mockTool) Meta() llm.ToolMeta               { return t.meta }
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

	m, err := NewManager("test", "0.0.0", WithTools(&mockTool{name: "my_tool", description: "A test tool"}))
	assert.NoError(err)

	meta, err := m.GetTool(context.TODO(), "my_tool")
	assert.NoError(err)
	assert.Equal("my_tool", meta.Name)
	assert.Equal("A test tool", meta.Description)
}

// Test GetTool with unknown name returns not found
func Test_tool_002(t *testing.T) {
	assert := assert.New(t)

	m, err := NewManager("test", "0.0.0")
	assert.NoError(err)

	_, err = m.GetTool(context.TODO(), "nonexistent")
	assert.ErrorIs(err, schema.ErrNotFound)
}

// Test GetTool includes JSON schema when present
func Test_tool_003(t *testing.T) {
	assert := assert.New(t)

	s, err := jsonschema.FromJSON(json.RawMessage(`{"type":"object","description":"input schema"}`))
	assert.NoError(err)
	destructive := true
	openWorld := true
	m, err := NewManager("test", "0.0.0", WithTools(&mockTool{name: "schema_tool", description: "Has schema", input: s, output: jsonschema.MustFor[string](), meta: llm.ToolMeta{Title: "Schema Tool", DestructiveHint: &destructive, OpenWorldHint: &openWorld}}))
	assert.NoError(err)

	meta, err := m.GetTool(context.TODO(), "schema_tool")
	assert.NoError(err)
	assert.NotNil(meta.Input)
	assert.Contains(string(meta.Input), `"object"`)
	assert.NotNil(meta.Output)
	assert.Contains(string(meta.Output), `"string"`)
	assert.Equal("Schema Tool", meta.Title)
	assert.NotNil(meta.Hints)
	assert.Equal([]string{"destructive", "openworld"}, meta.Hints)
}

// Test GetTool omits schema when tool has none
func Test_tool_004(t *testing.T) {
	assert := assert.New(t)

	m, err := NewManager("test", "0.0.0", WithTools(&mockTool{name: "no_schema", description: "No schema"}))
	assert.NoError(err)

	meta, err := m.GetTool(context.TODO(), "no_schema")
	assert.NoError(err)
	assert.Nil(meta.Input)
}

// Test CallTool executes a tool and returns the result
func Test_tool_005(t *testing.T) {
	assert := assert.New(t)

	m, err := NewManager("test", "0.0.0", WithTools(&mockTool{
		name:        "echo_tool",
		description: "Echoes input",
		runFn: func(_ context.Context, input json.RawMessage) (any, error) {
			data, _ := json.Marshal(map[string]string{"echoed": string(input)})
			return tool.NewJSONResource(data), nil
		},
	}))
	assert.NoError(err)

	resp, err := m.CallTool(context.TODO(), "echo_tool", json.RawMessage(`"hello"`))
	assert.NoError(err)
	assert.Equal("echo_tool", resp.Tool)
	assert.Contains(string(resp.Result), `"echoed"`)
}

// Test CallTool with unknown tool returns not found
func Test_tool_006(t *testing.T) {
	assert := assert.New(t)

	m, err := NewManager("test", "0.0.0")
	assert.NoError(err)

	_, err = m.CallTool(context.TODO(), "nonexistent", nil)
	assert.ErrorIs(err, schema.ErrNotFound)
}

// Test CallTool with nil input
func Test_tool_007(t *testing.T) {
	assert := assert.New(t)

	m, err := NewManager("test", "0.0.0", WithTools(&mockTool{
		name: "nil_input",
		runFn: func(_ context.Context, input json.RawMessage) (any, error) {
			return tool.NewJSONResource([]byte(`"ok"`)), nil
		},
	}))
	assert.NoError(err)

	resp, err := m.CallTool(context.TODO(), "nil_input", nil)
	assert.NoError(err)
	assert.Equal(`"ok"`, string(resp.Result))
}

// Test CallTool propagates tool execution errors
func Test_tool_008(t *testing.T) {
	assert := assert.New(t)

	m, err := NewManager("test", "0.0.0", WithTools(&mockTool{
		name: "fail_tool",
		runFn: func(_ context.Context, _ json.RawMessage) (any, error) {
			return nil, schema.ErrBadParameter.With("bad input")
		},
	}))
	assert.NoError(err)

	_, err = m.CallTool(context.TODO(), "fail_tool", json.RawMessage(`{}`))
	assert.ErrorIs(err, schema.ErrBadParameter)
}

// Test ListTools returns all tools sorted by name
func Test_tool_009(t *testing.T) {
	assert := assert.New(t)

	m, err := NewManager("test", "0.0.0", WithTools(
		&mockTool{name: "charlie", description: "C"},
		&mockTool{name: "alpha", description: "A"},
		&mockTool{name: "bravo", description: "B"},
	))
	assert.NoError(err)

	resp, err := m.ListTools(context.TODO(), schema.ToolListRequest{})
	assert.NoError(err)
	assert.Equal(uint(3), resp.Count)
	assert.Equal("alpha", resp.Body[0].Name)
	assert.Equal("bravo", resp.Body[1].Name)
	assert.Equal("charlie", resp.Body[2].Name)
}

// Test ListTools with limit
func Test_tool_010(t *testing.T) {
	assert := assert.New(t)

	m, err := NewManager("test", "0.0.0", WithTools(
		&mockTool{name: "alpha"},
		&mockTool{name: "bravo"},
		&mockTool{name: "charlie"},
	))
	assert.NoError(err)

	limit := uint(2)
	resp, err := m.ListTools(context.TODO(), schema.ToolListRequest{Limit: &limit})
	assert.NoError(err)
	assert.Equal(uint(3), resp.Count)
	assert.Len(resp.Body, 2)
	assert.Equal("alpha", resp.Body[0].Name)
	assert.Equal("bravo", resp.Body[1].Name)
}

// Test ListTools with offset
func Test_tool_011(t *testing.T) {
	assert := assert.New(t)

	m, err := NewManager("test", "0.0.0", WithTools(
		&mockTool{name: "alpha"},
		&mockTool{name: "bravo"},
		&mockTool{name: "charlie"},
	))
	assert.NoError(err)

	resp, err := m.ListTools(context.TODO(), schema.ToolListRequest{Offset: 1})
	assert.NoError(err)
	assert.Equal(uint(3), resp.Count)
	assert.Len(resp.Body, 2)
	assert.Equal("bravo", resp.Body[0].Name)
}

// Test ListTools with limit and offset
func Test_tool_012(t *testing.T) {
	assert := assert.New(t)

	m, err := NewManager("test", "0.0.0", WithTools(
		&mockTool{name: "alpha"},
		&mockTool{name: "bravo"},
		&mockTool{name: "charlie"},
	))
	assert.NoError(err)

	limit := uint(1)
	resp, err := m.ListTools(context.TODO(), schema.ToolListRequest{Limit: &limit, Offset: 1})
	assert.NoError(err)
	assert.Equal(uint(3), resp.Count)
	assert.Len(resp.Body, 1)
	assert.Equal("bravo", resp.Body[0].Name)
}

// Test ListTools empty
func Test_tool_013(t *testing.T) {
	assert := assert.New(t)

	m, err := NewManager("test", "0.0.0")
	assert.NoError(err)

	resp, err := m.ListTools(context.TODO(), schema.ToolListRequest{})
	assert.NoError(err)
	assert.Equal(uint(0), resp.Count)
	assert.Empty(resp.Body)
}

// Test ListTools offset beyond total
func Test_tool_014(t *testing.T) {
	assert := assert.New(t)

	m, err := NewManager("test", "0.0.0", WithTools(&mockTool{name: "alpha"}))
	assert.NoError(err)

	resp, err := m.ListTools(context.TODO(), schema.ToolListRequest{Offset: 10})
	assert.NoError(err)
	assert.Equal(uint(1), resp.Count)
	assert.Empty(resp.Body)
}

// Test ListTools includes input schema
func Test_tool_015(t *testing.T) {
	assert := assert.New(t)

	m, err := NewManager("test", "0.0.0", WithTools(&mockTool{name: "typed_tool", input: jsonschema.MustFor[string](), output: jsonschema.MustFor[int]()}))
	assert.NoError(err)

	resp, err := m.ListTools(context.TODO(), schema.ToolListRequest{})
	assert.NoError(err)
	assert.Len(resp.Body, 1)
	assert.NotNil(resp.Body[0].Input)
	assert.Contains(string(resp.Body[0].Input), `"string"`)
	assert.NotNil(resp.Body[0].Output)
	assert.Contains(string(resp.Body[0].Output), `"integer"`)
}

// Test CallTool result marshalling
func Test_tool_016(t *testing.T) {
	assert := assert.New(t)

	type weatherResult struct {
		Temp float64 `json:"temp"`
		Unit string  `json:"unit"`
	}

	m, err := NewManager("test", "0.0.0", WithTools(&mockTool{
		name: "get_weather",
		runFn: func(_ context.Context, _ json.RawMessage) (any, error) {
			data, _ := json.Marshal(weatherResult{Temp: 22.5, Unit: "celsius"})
			return tool.NewJSONResource(data), nil
		},
	}))
	assert.NoError(err)

	resp, err := m.CallTool(context.TODO(), "get_weather", nil)
	assert.NoError(err)
	assert.Equal("get_weather", resp.Tool)

	var result weatherResult
	assert.NoError(json.Unmarshal(resp.Result, &result))
	assert.Equal(22.5, result.Temp)
	assert.Equal("celsius", result.Unit)
}
