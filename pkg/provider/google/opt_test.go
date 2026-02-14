package google

import (
	"context"
	"encoding/json"
	"testing"

	// Packages
	jsonschema "github.com/google/jsonschema-go/jsonschema"
	opt "github.com/mutablelogic/go-llm/pkg/opt"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	"github.com/mutablelogic/go-llm/pkg/tool"
	assert "github.com/stretchr/testify/assert"
)

///////////////////////////////////////////////////////////////////////////////
// MOCK TOOL

// mockTool implements tool.Tool for testing
type mockTool struct {
	name        string
	description string
	schema      *jsonschema.Schema
}

func newMockTool(name, description string) *mockTool {
	return &mockTool{
		name:        name,
		description: description,
		schema: &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"location": {Type: "string", Description: "The city name"},
			},
		},
	}
}

func (m *mockTool) Name() string                                          { return m.name }
func (m *mockTool) Description() string                                   { return m.description }
func (m *mockTool) Schema() (*jsonschema.Schema, error)                   { return m.schema, nil }
func (m *mockTool) Run(_ context.Context, _ json.RawMessage) (any, error) { return "mock result", nil }

///////////////////////////////////////////////////////////////////////////////
// OPTION VALIDATION

func Test_opt_validation_001(t *testing.T) {
	// Temperature out of range (Google allows 0–2)
	_, err := opt.Apply(WithTemperature(2.5))
	assert.Error(t, err)

	_, err = opt.Apply(WithTemperature(-0.1))
	assert.Error(t, err)

	// Valid boundary values
	_, err = opt.Apply(WithTemperature(0.0))
	assert.NoError(t, err)

	_, err = opt.Apply(WithTemperature(2.0))
	assert.NoError(t, err)
}

func Test_opt_validation_002(t *testing.T) {
	// Max tokens below minimum
	_, err := opt.Apply(WithMaxTokens(0))
	assert.Error(t, err)
}

func Test_opt_validation_003(t *testing.T) {
	// TopK below minimum
	_, err := opt.Apply(WithTopK(0))
	assert.Error(t, err)
}

func Test_opt_validation_004(t *testing.T) {
	// TopP out of range
	_, err := opt.Apply(WithTopP(1.5))
	assert.Error(t, err)

	_, err = opt.Apply(WithTopP(-0.1))
	assert.Error(t, err)
}

func Test_opt_validation_005(t *testing.T) {
	// Empty stop sequences
	_, err := opt.Apply(WithStopSequences())
	assert.Error(t, err)
}

func Test_opt_validation_006(t *testing.T) {
	// Nil JSON schema
	_, err := opt.Apply(WithJSONOutput(nil))
	assert.Error(t, err)
}

///////////////////////////////////////////////////////////////////////////////
// TOOLKIT — geminiFunctionDeclsFromToolkit

func Test_opt_toolkit_001(t *testing.T) {
	// Test geminiFunctionDeclsFromToolkit with a single mock tool
	assert := assert.New(t)

	tk, err := tool.NewToolkit(newMockTool("get_weather", "Get current weather"))
	assert.NoError(err)

	decls := geminiFunctionDeclsFromToolkit(tk)
	assert.Len(decls, 1)
	assert.Equal("get_weather", decls[0].Name)
	assert.Equal("Get current weather", decls[0].Description)
	assert.NotNil(decls[0].ParametersJSONSchema)

	// Verify the schema has the expected structure
	props, ok := decls[0].ParametersJSONSchema["properties"].(map[string]any)
	assert.True(ok)
	assert.Contains(props, "location")
}

func Test_opt_toolkit_002(t *testing.T) {
	// Test geminiFunctionDeclsFromToolkit with multiple tools
	assert := assert.New(t)

	tk, err := tool.NewToolkit(
		newMockTool("get_weather", "Get weather"),
		newMockTool("search_web", "Search the web"),
	)
	assert.NoError(err)

	decls := geminiFunctionDeclsFromToolkit(tk)
	assert.Len(decls, 2)

	names := make(map[string]bool)
	for _, d := range decls {
		names[d.Name] = true
	}
	assert.True(names["get_weather"])
	assert.True(names["search_web"])
}

func Test_opt_toolkit_003(t *testing.T) {
	// Test toolkit tools appear in generateRequestFromOpts
	assert := assert.New(t)

	tk, err := tool.NewToolkit(newMockTool("get_weather", "Get weather"))
	assert.NoError(err)

	msg := &schema.Message{Role: "user", Content: []schema.ContentBlock{{Text: strPtr("Hi")}}}
	session := schema.Conversation{msg}
	o, err := opt.Apply(tool.WithToolkit(tk))
	assert.NoError(err)

	req, err := generateRequestFromOpts("gemini-2.0-flash", &session, o)
	assert.NoError(err)
	assert.Len(req.Tools, 1)
	assert.Len(req.Tools[0].FunctionDeclarations, 1)
	assert.Equal("get_weather", req.Tools[0].FunctionDeclarations[0].Name)
}

func Test_opt_toolkit_004(t *testing.T) {
	// Test empty toolkit produces no tools in request
	assert := assert.New(t)

	tk, err := tool.NewToolkit()
	assert.NoError(err)

	decls := geminiFunctionDeclsFromToolkit(tk)
	assert.Empty(decls)
}

func Test_opt_toolkit_005(t *testing.T) {
	// Test empty toolkit does not add tools block to request
	assert := assert.New(t)

	tk, err := tool.NewToolkit()
	assert.NoError(err)

	msg := &schema.Message{Role: "user", Content: []schema.ContentBlock{{Text: strPtr("Hi")}}}
	session := schema.Conversation{msg}
	o, err := opt.Apply(tool.WithToolkit(tk))
	assert.NoError(err)

	req, err := generateRequestFromOpts("gemini-2.0-flash", &session, o)
	assert.NoError(err)
	assert.Nil(req.Tools)
}
