package schema_test

import (
	"encoding/json"
	"errors"
	"testing"

	// Packages
	schema "github.com/mutablelogic/go-llm/provider/schema"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

func Test_NewToolResult(t *testing.T) {
	t.Run("Value", func(t *testing.T) {
		block := schema.NewToolResult("call_1", "get_weather", map[string]int{"temperature_c": 18})
		require.NotNil(t, block.ToolResult)
		assert.Equal(t, "call_1", block.ToolResult.ID)
		assert.Equal(t, "get_weather", block.ToolResult.Name)
		assert.JSONEq(t, `{"temperature_c":18}`, string(block.ToolResult.Content))
		assert.False(t, block.ToolResult.IsError)
	})

	t.Run("Error", func(t *testing.T) {
		block := schema.NewToolResult("call_1", "get_weather", errors.New("no \"city\"\x00"))
		require.NotNil(t, block.ToolResult)
		assert.Equal(t, "call_1", block.ToolResult.ID)
		assert.Equal(t, "get_weather", block.ToolResult.Name)
		assert.True(t, block.ToolResult.IsError)

		// The content is a valid JSON string, even with control characters
		var message string
		require.NoError(t, json.Unmarshal(block.ToolResult.Content, &message))
		assert.Equal(t, "no \"city\"\x00", message)
	})

	t.Run("NotEncodable", func(t *testing.T) {
		block := schema.NewToolResult("call_1", "get_weather", make(chan int))
		require.NotNil(t, block.ToolResult)
		assert.Equal(t, "call_1", block.ToolResult.ID)
		assert.True(t, block.ToolResult.IsError)
		assert.True(t, json.Valid(block.ToolResult.Content))
	})
}
