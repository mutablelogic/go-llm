package anthropic

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
// PAGINATION OPTIONS

func Test_opt_pagination_001(t *testing.T) {
	// Test WithAfterId
	assert := assert.New(t)
	o, err := opt.Apply(WithAfterId("model_abc"))
	assert.NoError(err)
	assert.Equal("model_abc", o.GetString(opt.AfterIdKey))
}

func Test_opt_pagination_002(t *testing.T) {
	// Test WithBeforeId
	assert := assert.New(t)
	o, err := opt.Apply(WithBeforeId("model_xyz"))
	assert.NoError(err)
	assert.Equal("model_xyz", o.GetString(opt.BeforeIdKey))
}

func Test_opt_pagination_003(t *testing.T) {
	// Test WithLimit
	assert := assert.New(t)
	o, err := opt.Apply(WithLimit(25))
	assert.NoError(err)
	assert.Equal(uint(25), o.GetUint(opt.LimitKey))
}

func Test_opt_pagination_004(t *testing.T) {
	// Test combining pagination options
	assert := assert.New(t)
	o, err := opt.Apply(WithAfterId("cursor_1"), WithLimit(10))
	assert.NoError(err)
	assert.Equal("cursor_1", o.GetString(opt.AfterIdKey))
	assert.Equal(uint(10), o.GetUint(opt.LimitKey))
}

///////////////////////////////////////////////////////////////////////////////
// TOOL CHOICE OPTIONS

func Test_opt_toolchoice_001(t *testing.T) {
	// Test WithToolChoiceAny
	assert := assert.New(t)
	o, err := opt.Apply(WithToolChoiceAny())
	assert.NoError(err)
	assert.Equal("any", o.GetString(opt.ToolChoiceKey))
}

func Test_opt_toolchoice_002(t *testing.T) {
	// Test WithToolChoiceNone
	assert := assert.New(t)
	o, err := opt.Apply(WithToolChoiceNone())
	assert.NoError(err)
	assert.Equal("none", o.GetString(opt.ToolChoiceKey))
}

func Test_opt_toolchoice_003(t *testing.T) {
	// Test WithToolChoice with empty name fails
	_, err := opt.Apply(WithToolChoice(""))
	assert.Error(t, err)
}

func Test_opt_toolchoice_004(t *testing.T) {
	// Test tool choice any in request
	assert := assert.New(t)

	msg := &schema.Message{Role: "user", Content: []schema.ContentBlock{{Text: strPtr("Hi")}}}
	session := schema.Conversation{msg}
	o, err := opt.Apply(WithToolChoiceAny())
	assert.NoError(err)

	req, err := generateRequestFromOpts("claude-sonnet-4-20250514", &session, o)
	assert.NoError(err)
	assert.NotNil(req.ToolChoice)
	assert.Equal("any", req.ToolChoice.Type)
}

func Test_opt_toolchoice_005(t *testing.T) {
	// Test tool choice none in request
	assert := assert.New(t)

	msg := &schema.Message{Role: "user", Content: []schema.ContentBlock{{Text: strPtr("Hi")}}}
	session := schema.Conversation{msg}
	o, err := opt.Apply(WithToolChoiceNone())
	assert.NoError(err)

	req, err := generateRequestFromOpts("claude-sonnet-4-20250514", &session, o)
	assert.NoError(err)
	assert.NotNil(req.ToolChoice)
	assert.Equal("none", req.ToolChoice.Type)
}

///////////////////////////////////////////////////////////////////////////////
// TOOLKIT (via anthropicToolsFromTools)

func Test_opt_toolkit_001(t *testing.T) {
	// Test anthropicToolsFromTools with a single mock tool
	assert := assert.New(t)

	tk, err := tool.NewToolkit(newMockTool("get_weather", "Get current weather"))
	assert.NoError(err)

	tools, err := anthropicToolsFromTools(tk.Tools())
	assert.NoError(err)
	assert.Len(tools, 1)

	var decoded map[string]any
	assert.NoError(json.Unmarshal(tools[0], &decoded))
	assert.Equal("get_weather", decoded["name"])
	assert.Equal("Get current weather", decoded["description"])
	assert.NotNil(decoded["input_schema"])

	// Verify input_schema has the expected structure
	inputSchema, ok := decoded["input_schema"].(map[string]any)
	assert.True(ok)
	assert.Equal("object", inputSchema["type"])
	props, ok := inputSchema["properties"].(map[string]any)
	assert.True(ok)
	assert.Contains(props, "location")
}

func Test_opt_toolkit_002(t *testing.T) {
	// Test anthropicToolsFromTools with multiple tools
	assert := assert.New(t)

	tk, err := tool.NewToolkit(
		newMockTool("get_weather", "Get weather"),
		newMockTool("search_web", "Search the web"),
	)
	assert.NoError(err)

	tools, err := anthropicToolsFromTools(tk.Tools())
	assert.NoError(err)
	assert.Len(tools, 2)

	// Collect names from decoded tools
	names := make(map[string]bool)
	for _, raw := range tools {
		var decoded map[string]any
		assert.NoError(json.Unmarshal(raw, &decoded))
		names[decoded["name"].(string)] = true
	}
	assert.True(names["get_weather"])
	assert.True(names["search_web"])
}

func Test_opt_toolkit_003(t *testing.T) {
	// Test toolkit tools appear in generateRequestFromOpts via WithToolkit
	assert := assert.New(t)

	tk, err := tool.NewToolkit(newMockTool("get_weather", "Get weather"))
	assert.NoError(err)

	msg := &schema.Message{Role: "user", Content: []schema.ContentBlock{{Text: strPtr("Hi")}}}
	session := schema.Conversation{msg}
	o, err := opt.Apply(tool.WithToolkit(tk))
	assert.NoError(err)

	req, err := generateRequestFromOpts("claude-sonnet-4-20250514", &session, o)
	assert.NoError(err)
	assert.Len(req.Tools, 1)

	var decoded map[string]any
	assert.NoError(json.Unmarshal(req.Tools[0], &decoded))
	assert.Equal("get_weather", decoded["name"])
}

func Test_opt_toolkit_004(t *testing.T) {
	// Test toolkit with tool choice in request
	assert := assert.New(t)

	tk, err := tool.NewToolkit(
		newMockTool("get_weather", "Get weather"),
		newMockTool("search_web", "Search the web"),
	)
	assert.NoError(err)

	msg := &schema.Message{Role: "user", Content: []schema.ContentBlock{{Text: strPtr("Hi")}}}
	session := schema.Conversation{msg}
	o, err := opt.Apply(tool.WithToolkit(tk), WithToolChoice("get_weather"))
	assert.NoError(err)

	req, err := generateRequestFromOpts("claude-sonnet-4-20250514", &session, o)
	assert.NoError(err)
	assert.Len(req.Tools, 2)
	assert.NotNil(req.ToolChoice)
	assert.Equal("tool", req.ToolChoice.Type)
	assert.Equal("get_weather", req.ToolChoice.Name)
}

func Test_opt_toolkit_005(t *testing.T) {
	// Test empty toolkit produces no tools
	assert := assert.New(t)

	tk, err := tool.NewToolkit()
	assert.NoError(err)

	tools, err := anthropicToolsFromTools(tk.Tools())
	assert.NoError(err)
	assert.Empty(tools)
}
