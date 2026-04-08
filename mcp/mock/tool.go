// Package mock provides configurable mock implementations of interfaces for
// use in tests across the go-llm module.
package mock

import (
	"context"
	"encoding/json"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	tool "github.com/mutablelogic/go-llm/toolkit/tool"
	jsonschema "github.com/mutablelogic/go-server/pkg/jsonschema"
)

// MockTool is a configurable implementation of llm.Tool for use in tests.
// Set Name_, Description_, and Result_ before registering it on a server.
// RunFn, if set, overrides Result_ and is called with the raw JSON input.
type MockTool struct {
	tool.Base
	Name_        string
	Description_ string
	InputSchema_ *jsonschema.Schema
	Meta_        llm.ToolMeta
	Result_      any
	RunFn        func(ctx context.Context, input json.RawMessage) (any, error)
}

var _ llm.Tool = (*MockTool)(nil)

func (m *MockTool) Name() string        { return m.Name_ }
func (m *MockTool) Description() string { return m.Description_ }
func (m *MockTool) Meta() llm.ToolMeta  { return m.Meta_ }

func (m *MockTool) InputSchema() *jsonschema.Schema {
	if m.InputSchema_ != nil {
		return m.InputSchema_
	}
	return jsonschema.MustFor[map[string]any]()
}

func (m *MockTool) Run(ctx context.Context, input json.RawMessage) (any, error) {
	if m.RunFn != nil {
		return m.RunFn(ctx, input)
	}
	return m.Result_, nil
}
