package weatherapi

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewTools(t *testing.T) {
	assert := assert.New(t)

	// Test with invalid API key
	tools, err := NewTools("")
	assert.Error(err)
	assert.Nil(tools)

	// Test with valid API key (but we won't make actual requests)
	tools2, err2 := NewTools("test-api-key")
	assert.NoError(err2)
	assert.Len(tools2, 3)

	// Check tool names
	names := make([]string, 0, len(tools2))
	for _, tool := range tools2 {
		names = append(names, tool.Name())
	}

	assert.Contains(names, "weatherapi_current")
	assert.Contains(names, "weatherapi_forecast")
	assert.Contains(names, "weatherapi_alerts")
}

func TestCurrentWeatherToolInterface(t *testing.T) {
	assert := assert.New(t)

	tools2, err2 := NewTools("test-api-key")
	assert.NoError(err2)

	tool := tools2[0]

	// Test tool methods
	assert.Equal("weatherapi_current", tool.Name())
	assert.NotEmpty(tool.Description())

	// Test schema
	schema, err := tool.Schema()
	assert.NoError(err)
	assert.NotNil(schema)
	assert.Contains(schema.Properties, "query")

	// Test invalid input (missing required query)
	input := json.RawMessage(`{}`)
	result, err := tool.Run(context.Background(), input)
	assert.Error(err)
	assert.Nil(result)

	// Test valid input (won't actually call API without valid key)
	input = json.RawMessage(`{"query":"London"}`)
	result, err = tool.Run(context.Background(), input)
	// Will error due to invalid API key, but that's expected
	assert.Error(err)
}

func TestForecastWeatherToolInterface(t *testing.T) {
	assert := assert.New(t)

	tools2, err2 := NewTools("test-api-key")
	assert.NoError(err2)

	tool := tools2[1]

	// Test tool methods
	assert.Equal("weatherapi_forecast", tool.Name())
	assert.NotEmpty(tool.Description())

	// Test schema
	schema, err := tool.Schema()
	assert.NoError(err)
	assert.NotNil(schema)
	assert.Contains(schema.Properties, "query")
	assert.Contains(schema.Properties, "days")

	// Test invalid input (missing required fields)
	input := json.RawMessage(`{}`)
	result, err := tool.Run(context.Background(), input)
	assert.Error(err)
	assert.Nil(result)

	// Test invalid days (must be 1-14)
	input = json.RawMessage(`{"query":"London","days":20}`)
	result, err = tool.Run(context.Background(), input)
	assert.Error(err)
	assert.Nil(result)

	// Test valid input
	input = json.RawMessage(`{"query":"London","days":3}`)
	result, err = tool.Run(context.Background(), input)
	// Will error due to invalid API key, but that's expected
	assert.Error(err)
}

func TestAlertsWeatherToolInterface(t *testing.T) {
	assert := assert.New(t)

	tools2, err2 := NewTools("test-api-key")
	assert.NoError(err2)

	tool := tools2[2]

	// Test tool methods
	assert.Equal("weatherapi_alerts", tool.Name())
	assert.NotEmpty(tool.Description())

	// Test schema
	schema, err := tool.Schema()
	assert.NoError(err)
	assert.NotNil(schema)
	assert.Contains(schema.Properties, "query")

	// Test invalid input (missing required query)
	input := json.RawMessage(`{}`)
	result, err := tool.Run(context.Background(), input)
	assert.Error(err)
	assert.Nil(result)

	// Test valid input
	input = json.RawMessage(`{"query":"London"}`)
	result, err = tool.Run(context.Background(), input)
	// Will error due to invalid API key, but that's expected
	assert.Error(err)
}
