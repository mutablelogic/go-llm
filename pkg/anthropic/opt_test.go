package anthropic_test

import (
	"encoding/json"
	"testing"

	// Packages
	"github.com/google/jsonschema-go/jsonschema"
	anthropic "github.com/mutablelogic/go-llm/pkg/anthropic"
	opt "github.com/mutablelogic/go-llm/pkg/opt"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

func Test_opt_001(t *testing.T) {
	assert := assert.New(t)

	// Apply with WithAfterId
	opts, err := opt.Apply(anthropic.WithAfterId("abc123"))
	assert.NoError(err)
	assert.NotNil(opts)

	query := opts.Query("after_id")
	assert.Equal("abc123", query.Get("after_id"))
}

func Test_opt_002(t *testing.T) {
	assert := assert.New(t)

	// Apply with WithBeforeId
	opts, err := opt.Apply(anthropic.WithBeforeId("xyz789"))
	assert.NoError(err)
	assert.NotNil(opts)

	query := opts.Query("before_id")
	assert.Equal("xyz789", query.Get("before_id"))
}

func Test_opt_003(t *testing.T) {
	assert := assert.New(t)

	// Apply with WithLimit
	opts, err := opt.Apply(anthropic.WithLimit(50))
	assert.NoError(err)
	assert.NotNil(opts)

	query := opts.Query("limit")
	assert.Equal("50", query.Get("limit"))
}

func Test_opt_004(t *testing.T) {
	assert := assert.New(t)

	// Apply with multiple anthropic options
	opts, err := opt.Apply(
		anthropic.WithAfterId("abc123"),
		anthropic.WithBeforeId("xyz789"),
		anthropic.WithLimit(25),
	)
	assert.NoError(err)
	assert.NotNil(opts)

	query := opts.Query("after_id", "before_id", "limit")
	assert.Equal("abc123", query.Get("after_id"))
	assert.Equal("xyz789", query.Get("before_id"))
	assert.Equal("25", query.Get("limit"))
}

func Test_opt_005(t *testing.T) {
	assert := assert.New(t)

	session := new(schema.Session)
	session.Append(schema.StringMessage("user", "Hello"))

	req, err := anthropic.MessagesRequest("claude-haiku-4-5-20251001", session)
	require.NoError(t, err)

	data, err := json.MarshalIndent(req, "", "  ")
	require.NoError(t, err)

	t.Log(string(data))
	assert.Contains(string(data), `"model": "claude-haiku-4-5-20251001"`)
}

func Test_opt_006(t *testing.T) {
	assert := assert.New(t)

	session := new(schema.Session)
	session.Append(schema.StringMessage("user", "Hello"))

	req, err := anthropic.MessagesRequest("claude-haiku-4-5-20251001", session,
		anthropic.WithUser("test-user-123"),
		anthropic.WithServiceTier("auto"),
	)
	require.NoError(t, err)

	data, err := json.MarshalIndent(req, "", "  ")
	require.NoError(t, err)

	t.Log(string(data))
	assert.Contains(string(data), `"user_id": "test-user-123"`)
	assert.Contains(string(data), `"service_tier": "auto"`)
}

func Test_opt_007(t *testing.T) {
	assert := assert.New(t)

	session := new(schema.Session)
	session.Append(schema.StringMessage("user", "Hello"))

	req, err := anthropic.MessagesRequest("claude-haiku-4-5-20251001", session,
		anthropic.WithSystemPrompt("You are a helpful assistant"),
	)
	require.NoError(t, err)

	data, err := json.MarshalIndent(req, "", "  ")
	require.NoError(t, err)

	t.Log(string(data))
	assert.Contains(string(data), `"system": "You are a helpful assistant"`)
	assert.NotContains(string(data), `"cache_control"`)
}

func Test_opt_008(t *testing.T) {
	assert := assert.New(t)

	session := new(schema.Session)
	session.Append(schema.StringMessage("user", "Hello"))

	req, err := anthropic.MessagesRequest("claude-haiku-4-5-20251001", session,
		anthropic.WithCachedSystemPrompt("You are a helpful assistant"),
	)
	require.NoError(t, err)

	data, err := json.MarshalIndent(req, "", "  ")
	require.NoError(t, err)

	t.Log(string(data))
	assert.Contains(string(data), `"text": "You are a helpful assistant"`)
	assert.Contains(string(data), `"cache_control"`)
	assert.Contains(string(data), `"type": "ephemeral"`)
}

func Test_opt_009(t *testing.T) {
	assert := assert.New(t)

	session := new(schema.Session)
	session.Append(schema.StringMessage("user", "Hello"))

	req, err := anthropic.MessagesRequest("claude-haiku-4-5-20251001", session,
		anthropic.WithStopSequences("STOP", "END"),
		anthropic.WithStream(),
	)
	require.NoError(t, err)

	data, err := json.MarshalIndent(req, "", "  ")
	require.NoError(t, err)

	t.Log(string(data))
	assert.Contains(string(data), `"stop_sequences"`)
	assert.Contains(string(data), `"STOP"`)
	assert.Contains(string(data), `"END"`)
	assert.Contains(string(data), `"stream": true`)
}

func Test_opt_010(t *testing.T) {
	assert := assert.New(t)

	session := new(schema.Session)
	session.Append(schema.StringMessage("user", "Hello"))

	req, err := anthropic.MessagesRequest("claude-haiku-4-5-20251001", session,
		anthropic.WithTemperature(0.7),
	)
	require.NoError(t, err)

	data, err := json.MarshalIndent(req, "", "  ")
	require.NoError(t, err)

	t.Log(string(data))
	assert.Contains(string(data), `"temperature": 0.7`)
}

func Test_opt_011(t *testing.T) {
	assert := assert.New(t)

	session := new(schema.Session)
	session.Append(schema.StringMessage("user", "Hello"))

	req, err := anthropic.MessagesRequest("claude-haiku-4-5-20251001", session,
		anthropic.WithThinking(10000),
	)
	require.NoError(t, err)

	data, err := json.MarshalIndent(req, "", "  ")
	require.NoError(t, err)

	t.Log(string(data))
	assert.Contains(string(data), `"thinking"`)
	assert.Contains(string(data), `"type": "enabled"`)
	assert.Contains(string(data), `"budget_tokens": 10000`)
}

func Test_opt_012(t *testing.T) {
	assert := assert.New(t)

	session := new(schema.Session)
	session.Append(schema.StringMessage("user", "Hello"))

	// WithThinking with budget below minimum should return error
	_, err := anthropic.MessagesRequest("claude-haiku-4-5-20251001", session,
		anthropic.WithThinking(500),
	)
	assert.Error(err)
	assert.Contains(err.Error(), "at least 1024")
}

func Test_opt_013(t *testing.T) {
	assert := assert.New(t)

	session := new(schema.Session)
	session.Append(schema.StringMessage("user", "Hello"))

	// WithMaxTokens with 0 should return error
	_, err := anthropic.MessagesRequest("claude-haiku-4-5-20251001", session,
		anthropic.WithMaxTokens(0),
	)
	assert.Error(err)
	assert.Contains(err.Error(), "at least 1")
}

func Test_opt_014(t *testing.T) {
	assert := assert.New(t)

	session := new(schema.Session)
	session.Append(schema.StringMessage("user", "Hello"))

	// WithMaxTokens with valid value should work
	req, err := anthropic.MessagesRequest("claude-haiku-4-5-20251001", session,
		anthropic.WithMaxTokens(2048),
	)
	require.NoError(t, err)

	data, err := json.MarshalIndent(req, "", "  ")
	require.NoError(t, err)

	t.Log(string(data))
	assert.Contains(string(data), `"max_tokens": 2048`)
}

func Test_opt_015(t *testing.T) {
	assert := assert.New(t)

	session := new(schema.Session)
	session.Append(schema.StringMessage("user", "Hello"))

	// WithStopSequences with no values should return error
	_, err := anthropic.MessagesRequest("claude-haiku-4-5-20251001", session,
		anthropic.WithStopSequences(),
	)
	assert.Error(err)
	assert.Contains(err.Error(), "at least one stop sequence")
}

func Test_opt_016(t *testing.T) {
	assert := assert.New(t)

	session := new(schema.Session)
	session.Append(schema.StringMessage("user", "Hello"))

	// WithTemperature below 0 should return error
	_, err := anthropic.MessagesRequest("claude-haiku-4-5-20251001", session,
		anthropic.WithTemperature(-0.1),
	)
	assert.Error(err)
	assert.Contains(err.Error(), "between 0.0 and 1.0")
}

func Test_opt_017(t *testing.T) {
	assert := assert.New(t)

	session := new(schema.Session)
	session.Append(schema.StringMessage("user", "Hello"))

	// WithTemperature above 1 should return error
	_, err := anthropic.MessagesRequest("claude-haiku-4-5-20251001", session,
		anthropic.WithTemperature(1.5),
	)
	assert.Error(err)
	assert.Contains(err.Error(), "between 0.0 and 1.0")
}

func Test_opt_018(t *testing.T) {
	assert := assert.New(t)

	session := new(schema.Session)
	session.Append(schema.StringMessage("user", "Hello"))

	// WithTopK with valid value should work
	req, err := anthropic.MessagesRequest("claude-haiku-4-5-20251001", session,
		anthropic.WithTopK(40),
	)
	require.NoError(t, err)

	data, err := json.MarshalIndent(req, "", "  ")
	require.NoError(t, err)

	t.Log(string(data))
	assert.Contains(string(data), `"top_k": 40`)
}

func Test_opt_019(t *testing.T) {
	assert := assert.New(t)

	session := new(schema.Session)
	session.Append(schema.StringMessage("user", "Hello"))

	// WithTopK with 0 should return error
	_, err := anthropic.MessagesRequest("claude-haiku-4-5-20251001", session,
		anthropic.WithTopK(0),
	)
	assert.Error(err)
	assert.Contains(err.Error(), "at least 1")
}

func Test_opt_020(t *testing.T) {
	assert := assert.New(t)

	session := new(schema.Session)
	session.Append(schema.StringMessage("user", "Hello"))

	// WithTopP with valid value should work
	req, err := anthropic.MessagesRequest("claude-haiku-4-5-20251001", session,
		anthropic.WithTopP(0.9),
	)
	require.NoError(t, err)

	data, err := json.MarshalIndent(req, "", "  ")
	require.NoError(t, err)

	t.Log(string(data))
	assert.Contains(string(data), `"top_p": 0.9`)
}

func Test_opt_021(t *testing.T) {
	assert := assert.New(t)

	session := new(schema.Session)
	session.Append(schema.StringMessage("user", "Hello"))

	// WithTopP below 0 should return error
	_, err := anthropic.MessagesRequest("claude-haiku-4-5-20251001", session,
		anthropic.WithTopP(-0.1),
	)
	assert.Error(err)
	assert.Contains(err.Error(), "between 0.0 and 1.0")
}

func Test_opt_022(t *testing.T) {
	assert := assert.New(t)

	session := new(schema.Session)
	session.Append(schema.StringMessage("user", "Hello"))

	// WithTopP above 1 should return error
	_, err := anthropic.MessagesRequest("claude-haiku-4-5-20251001", session,
		anthropic.WithTopP(1.5),
	)
	assert.Error(err)
	assert.Contains(err.Error(), "between 0.0 and 1.0")
}

func Test_opt_023(t *testing.T) {
	assert := assert.New(t)

	session := new(schema.Session)
	session.Append(schema.StringMessage("user", "Hello"))

	// WithOutputConfig with valid value should work
	req, err := anthropic.MessagesRequest("claude-haiku-4-5-20251001", session,
		anthropic.WithOutputConfig("high"),
	)
	require.NoError(t, err)

	data, err := json.MarshalIndent(req, "", "  ")
	require.NoError(t, err)

	t.Log(string(data))
	assert.Contains(string(data), `"output_config": "high"`)
}

func Test_opt_024(t *testing.T) {
	assert := assert.New(t)

	session := new(schema.Session)
	session.Append(schema.StringMessage("user", "Hello"))

	// WithOutputConfig with invalid value should return error
	_, err := anthropic.MessagesRequest("claude-haiku-4-5-20251001", session,
		anthropic.WithOutputConfig("invalid"),
	)
	assert.Error(err)
	assert.Contains(err.Error(), "'low', 'medium', or 'high'")
}

func Test_opt_025(t *testing.T) {
	assert := assert.New(t)

	session := new(schema.Session)
	session.Append(schema.StringMessage("user", "Hello"))

	// WithJSONOutput with a valid schema inferred from a Go type
	type Person struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}
	personSchema, err := jsonschema.For[Person](nil)
	require.NoError(t, err)

	req, err := anthropic.MessagesRequest("claude-haiku-4-5-20251001", session,
		anthropic.WithJSONOutput(personSchema),
	)
	require.NoError(t, err)

	data, err := json.MarshalIndent(req, "", "  ")
	require.NoError(t, err)

	t.Log(string(data))
	assert.Contains(string(data), `"output_format"`)
	assert.Contains(string(data), `"type": "json_schema"`)
	assert.Contains(string(data), `"json_schema"`)
	assert.Contains(string(data), `"name"`)
	assert.Contains(string(data), `"age"`)
}

func Test_opt_026(t *testing.T) {
	assert := assert.New(t)

	session := new(schema.Session)
	session.Append(schema.StringMessage("user", "Hello"))

	// WithJSONOutput with nil schema should return error
	_, err := anthropic.MessagesRequest("claude-haiku-4-5-20251001", session,
		anthropic.WithJSONOutput(nil),
	)
	assert.Error(err)
	assert.Contains(err.Error(), "schema is required")
}

func Test_opt_027(t *testing.T) {
	assert := assert.New(t)

	session := new(schema.Session)
	session.Append(schema.StringMessage("user", "Hello"))

	// WithToolChoiceAuto
	req, err := anthropic.MessagesRequest("claude-haiku-4-5-20251001", session,
		anthropic.WithToolChoiceAuto(),
	)
	require.NoError(t, err)

	data, err := json.MarshalIndent(req, "", "  ")
	require.NoError(t, err)

	t.Log(string(data))
	assert.Contains(string(data), `"tool_choice"`)
	assert.Contains(string(data), `"type": "auto"`)
}

func Test_opt_028(t *testing.T) {
	assert := assert.New(t)

	session := new(schema.Session)
	session.Append(schema.StringMessage("user", "Hello"))

	// WithToolChoiceAny
	req, err := anthropic.MessagesRequest("claude-haiku-4-5-20251001", session,
		anthropic.WithToolChoiceAny(),
	)
	require.NoError(t, err)

	data, err := json.MarshalIndent(req, "", "  ")
	require.NoError(t, err)

	t.Log(string(data))
	assert.Contains(string(data), `"tool_choice"`)
	assert.Contains(string(data), `"type": "any"`)
}

func Test_opt_029(t *testing.T) {
	assert := assert.New(t)

	session := new(schema.Session)
	session.Append(schema.StringMessage("user", "Hello"))

	// WithToolChoiceNone
	req, err := anthropic.MessagesRequest("claude-haiku-4-5-20251001", session,
		anthropic.WithToolChoiceNone(),
	)
	require.NoError(t, err)

	data, err := json.MarshalIndent(req, "", "  ")
	require.NoError(t, err)

	t.Log(string(data))
	assert.Contains(string(data), `"tool_choice"`)
	assert.Contains(string(data), `"type": "none"`)
}

func Test_opt_030(t *testing.T) {
	assert := assert.New(t)

	session := new(schema.Session)
	session.Append(schema.StringMessage("user", "Hello"))

	// WithToolChoice with specific tool name
	req, err := anthropic.MessagesRequest("claude-haiku-4-5-20251001", session,
		anthropic.WithToolChoice("get_weather"),
	)
	require.NoError(t, err)

	data, err := json.MarshalIndent(req, "", "  ")
	require.NoError(t, err)

	t.Log(string(data))
	assert.Contains(string(data), `"tool_choice"`)
	assert.Contains(string(data), `"type": "tool"`)
	assert.Contains(string(data), `"name": "get_weather"`)
}

func Test_opt_031(t *testing.T) {
	assert := assert.New(t)

	session := new(schema.Session)
	session.Append(schema.StringMessage("user", "Hello"))

	// WithToolChoice with empty name should return error
	_, err := anthropic.MessagesRequest("claude-haiku-4-5-20251001", session,
		anthropic.WithToolChoice(""),
	)
	assert.Error(err)
	assert.Contains(err.Error(), "tool name is required")
}

func Test_opt_032(t *testing.T) {
	assert := assert.New(t)

	session := new(schema.Session)
	session.Append(schema.StringMessage("user", "Hello"))

	// WithTool with valid schema
	type GetWeatherParams struct {
		Location string `json:"location" jsonschema:"description=The city and state"`
	}
	inputSchema, err := jsonschema.For[GetWeatherParams](nil)
	require.NoError(t, err)

	req, err := anthropic.MessagesRequest("claude-haiku-4-5-20251001", session,
		anthropic.WithTool("get_weather", "Get weather for a location", inputSchema),
	)
	require.NoError(t, err)

	data, err := json.MarshalIndent(req, "", "  ")
	require.NoError(t, err)

	t.Log(string(data))
	assert.Contains(string(data), `"tools"`)
	assert.Contains(string(data), `"name": "get_weather"`)
	assert.Contains(string(data), `"description": "Get weather for a location"`)
	assert.Contains(string(data), `"input_schema"`)
}

func Test_opt_033(t *testing.T) {
	assert := assert.New(t)

	session := new(schema.Session)
	session.Append(schema.StringMessage("user", "Hello"))

	// WithTool with empty name should return error
	type Params struct {
		Value string `json:"value"`
	}
	inputSchema, err := jsonschema.For[Params](nil)
	require.NoError(t, err)

	_, err = anthropic.MessagesRequest("claude-haiku-4-5-20251001", session,
		anthropic.WithTool("", "Some description", inputSchema),
	)
	assert.Error(err)
	assert.Contains(err.Error(), "tool name is required")
}

func Test_opt_034(t *testing.T) {
	assert := assert.New(t)

	session := new(schema.Session)
	session.Append(schema.StringMessage("user", "Hello"))

	// WithTool with nil schema should return error
	_, err := anthropic.MessagesRequest("claude-haiku-4-5-20251001", session,
		anthropic.WithTool("my_tool", "Some description", nil),
	)
	assert.Error(err)
	assert.Contains(err.Error(), "input schema is required")
}

func Test_opt_035(t *testing.T) {
	assert := assert.New(t)

	session := new(schema.Session)
	session.Append(schema.StringMessage("user", "Hello"))

	// Multiple WithTool calls should append tools
	type WeatherParams struct {
		Location string `json:"location"`
	}
	type TimeParams struct {
		Timezone string `json:"timezone"`
	}
	weatherSchema, err := jsonschema.For[WeatherParams](nil)
	require.NoError(t, err)
	timeSchema, err := jsonschema.For[TimeParams](nil)
	require.NoError(t, err)

	req, err := anthropic.MessagesRequest("claude-haiku-4-5-20251001", session,
		anthropic.WithTool("get_weather", "Get weather", weatherSchema),
		anthropic.WithTool("get_time", "Get current time", timeSchema),
	)
	require.NoError(t, err)

	data, err := json.MarshalIndent(req, "", "  ")
	require.NoError(t, err)

	t.Log(string(data))
	assert.Contains(string(data), `"name": "get_weather"`)
	assert.Contains(string(data), `"name": "get_time"`)
}

func Test_opt_036(t *testing.T) {
	assert := assert.New(t)

	session := new(schema.Session)
	session.Append(schema.StringMessage("user", "Hello"))

	// WithTool with empty description should still work (description is optional)
	type Params struct {
		Value string `json:"value"`
	}
	inputSchema, err := jsonschema.For[Params](nil)
	require.NoError(t, err)

	req, err := anthropic.MessagesRequest("claude-haiku-4-5-20251001", session,
		anthropic.WithTool("my_tool", "", inputSchema),
	)
	require.NoError(t, err)

	data, err := json.MarshalIndent(req, "", "  ")
	require.NoError(t, err)

	t.Log(string(data))
	assert.Contains(string(data), `"name": "my_tool"`)
	assert.NotContains(string(data), `"description"`)
}
