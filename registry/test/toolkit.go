package test

import (
	"context"
	"encoding/json"
	"testing"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	toolkit "github.com/mutablelogic/go-llm/toolkit"
	jsonschema "github.com/mutablelogic/go-server/pkg/jsonschema"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// tool is a tool for tests, with the name, which returns its input
type tool struct {
	toolkit.Base
	name string
}

var _ llm.Tool = tool{}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// NewToolkit returns a toolkit for tests, with a memory namespace, which has
// no tools, and a weather namespace, which has the current and forecast
// tools, or fails the test
func NewToolkit(t *testing.T) *toolkit.Toolkit {
	t.Helper()
	tools, err := toolkit.New(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err := tools.Add(toolkit.NewNamespace("memory", "")); err != nil {
		t.Fatal(err)
	} else if err := tools.Add(toolkit.NewNamespace("weather", "", tool{name: "current"}, tool{name: "forecast"})); err != nil {
		t.Fatal(err)
	}
	return tools
}

func (t tool) Name() string                    { return t.name }
func (t tool) Description() string             { return "A tool for tests, which returns its input" }
func (t tool) InputSchema() *jsonschema.Schema { return nil }
func (t tool) Run(_ context.Context, input json.RawMessage) (any, error) {
	return input, nil
}
