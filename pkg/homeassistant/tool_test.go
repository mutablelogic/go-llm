package homeassistant_test

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	// Packages
	opts "github.com/mutablelogic/go-client"
	homeassistant "github.com/mutablelogic/go-llm/pkg/homeassistant"
	"github.com/mutablelogic/go-llm/pkg/tool"
	assert "github.com/stretchr/testify/assert"
)

// Test NewTools creates all expected tools
func Test_tool_001(t *testing.T) {
	assert := assert.New(t)
	tools, err := homeassistant.NewTools(
		GetEndPoint(t), GetApiKey(t),
		opts.OptTrace(os.Stderr, true),
	)
	assert.NoError(err)
	assert.Len(tools, 7)

	names := make(map[string]bool)
	for _, tl := range tools {
		names[tl.Name()] = true
		// Verify each tool has a valid schema
		schema, err := tl.Schema()
		assert.NoError(err)
		assert.NotNil(schema)
		// Verify each tool has a description
		assert.NotEmpty(tl.Description())
	}

	assert.True(names["ha_get_states"])
	assert.True(names["ha_get_state"])
	assert.True(names["ha_call_service"])
	assert.True(names["ha_get_services"])
	assert.True(names["ha_set_state"])
	assert.True(names["ha_fire_event"])
	assert.True(names["ha_template"])
}

// Test ha_get_states returns entities
func Test_tool_002(t *testing.T) {
	assert := assert.New(t)
	tools, err := homeassistant.NewTools(
		GetEndPoint(t), GetApiKey(t),
		opts.OptTrace(os.Stderr, true),
	)
	assert.NoError(err)

	result := runTool(t, tools, "ha_get_states", nil)
	assert.NotNil(result)
	t.Logf("ha_get_states: got %d entities", len(result.([]any)))
}

// Test ha_get_states filtered by domain
func Test_tool_003(t *testing.T) {
	assert := assert.New(t)
	tools, err := homeassistant.NewTools(
		GetEndPoint(t), GetApiKey(t),
	)
	assert.NoError(err)

	result := runTool(t, tools, "ha_get_states", homeassistant.GetStatesRequest{
		Domain: "sun",
	})
	assert.NotNil(result)
}

// Test ha_get_state for sun.sun (always available)
func Test_tool_004(t *testing.T) {
	assert := assert.New(t)
	tools, err := homeassistant.NewTools(
		GetEndPoint(t), GetApiKey(t),
	)
	assert.NoError(err)

	result := runTool(t, tools, "ha_get_state", homeassistant.GetStateRequest{
		EntityId: "sun.sun",
	})
	assert.NotNil(result)
	t.Log("ha_get_state sun.sun:", result)
}

// Test ha_get_services for homeassistant domain
func Test_tool_005(t *testing.T) {
	assert := assert.New(t)
	tools, err := homeassistant.NewTools(
		GetEndPoint(t), GetApiKey(t),
	)
	assert.NoError(err)

	result := runTool(t, tools, "ha_get_services", homeassistant.GetServicesRequest{
		Domain: "homeassistant",
	})
	assert.NotNil(result)
	t.Log("ha_get_services homeassistant:", result)
}

// Test ha_template renders a template
func Test_tool_006(t *testing.T) {
	assert := assert.New(t)
	tools, err := homeassistant.NewTools(
		GetEndPoint(t), GetApiKey(t),
		opts.OptTrace(os.Stderr, true),
	)
	assert.NoError(err)

	result := runTool(t, tools, "ha_template", homeassistant.RenderTemplateRequest{
		Template: "It is {{ now() }}!",
	})
	assert.NotNil(result)
	t.Log("ha_template:", result)
}

///////////////////////////////////////////////////////////////////////////////
// HELPERS

func runTool(t *testing.T, tools []tool.Tool, name string, input any) any {
	t.Helper()
	for _, tl := range tools {
		if tl.Name() == name {
			var raw json.RawMessage
			if input != nil {
				data, err := json.Marshal(input)
				assert.NoError(t, err)
				raw = data
			}
			result, err := tl.Run(context.TODO(), raw)
			assert.NoError(t, err)
			return result
		}
	}
	t.Fatalf("tool %q not found", name)
	return nil
}
