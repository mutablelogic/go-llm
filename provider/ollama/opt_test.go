package ollama

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	jsonschema "github.com/mutablelogic/go-server/pkg/jsonschema"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

func TestApply(t *testing.T) {
	t.Run("None", func(t *testing.T) {
		var r chatRequest
		require.NoError(t, r.apply())
		assert.Equal(t, chatRequest{}, r)
	})

	t.Run("Options", func(t *testing.T) {
		var r chatRequest
		require.NoError(t, r.apply(
			llm.WithTemperature(0.5),
			llm.WithTopP(0.9),
			llm.WithTopK(40),
			llm.WithMaxTokens(100),
			llm.WithStopSequences("END"),
			llm.WithSeed(42),
			llm.WithPresencePenalty(0.1),
			llm.WithFrequencyPenalty(0.2),
			llm.WithContextWindow(8192),
		))
		assert.Equal(t, map[string]any{
			"temperature":       0.5,
			"top_p":             0.9,
			"top_k":             uint(40),
			"num_predict":       uint(100),
			"stop":              []string{"END"},
			"seed":              uint(42),
			"presence_penalty":  0.1,
			"frequency_penalty": 0.2,
			"num_ctx":           uint(8192),
		}, r.Options)
	})

	t.Run("SystemPrompt", func(t *testing.T) {
		r := chatRequest{Messages: []chatMessage{{Role: "user", Content: "Hello"}}}
		require.NoError(t, r.apply(llm.WithSystemPrompt("Be brief")))
		assert.Equal(t, []chatMessage{
			{Role: "system", Content: "Be brief"},
			{Role: "user", Content: "Hello"},
		}, r.Messages)
	})

	t.Run("Thinking", func(t *testing.T) {
		for name, test := range map[string]struct {
			opts []llm.Opt
			want any
		}{
			"Enabled":        {[]llm.Opt{llm.WithThinking(true)}, true},
			"Disabled":       {[]llm.Opt{llm.WithThinking(false)}, false},
			"Effort":         {[]llm.Opt{llm.WithThinkingEffort("low")}, "low"},
			"EnabledEffort":  {[]llm.Opt{llm.WithThinking(true), llm.WithThinkingEffort("high")}, "high"},
			"DisabledEffort": {[]llm.Opt{llm.WithThinking(false), llm.WithThinkingEffort("high")}, false},
		} {
			var r chatRequest
			require.NoError(t, r.apply(test.opts...), name)
			assert.Equal(t, test.want, r.Think, name)
		}
	})

	t.Run("JSONSchema", func(t *testing.T) {
		var r chatRequest
		require.NoError(t, r.apply(llm.WithJSONSchema(json.RawMessage(`{"type":"object"}`))))
		assert.JSONEq(t, `{"type":"object"}`, string(r.Format))
	})

	t.Run("KeepAlive", func(t *testing.T) {
		for duration, want := range map[time.Duration]string{
			5 * time.Minute: "5m0s",
			0:               "0s",
			-time.Second:    "-1s",
		} {
			var r chatRequest
			require.NoError(t, r.apply(llm.WithKeepAlive(duration)))
			assert.Equal(t, want, r.KeepAlive, duration)
		}
	})

	t.Run("ToolChoiceNone", func(t *testing.T) {
		// The tools are removed
		r := chatRequest{Tools: []chatTool{{Type: "function"}}}
		require.NoError(t, r.apply(llm.WithToolChoice("none")))
		assert.Nil(t, r.Tools)
	})

	t.Run("Unsupported", func(t *testing.T) {
		for _, opt := range []llm.Opt{
			llm.WithToolChoice("auto"),
			llm.WithOpt("unknown", true),
		} {
			var r chatRequest
			assert.ErrorIs(t, r.apply(llm.WithTemperature(0.5), opt), llm.ErrBadParameter)
		}
	})

	t.Run("WrongType", func(t *testing.T) {
		var r chatRequest
		assert.ErrorIs(t, r.apply(llm.WithOpt(llm.TemperatureKey, "hot")), llm.ErrBadParameter)
	})

	t.Run("Invalid", func(t *testing.T) {
		var r chatRequest
		assert.ErrorIs(t, r.apply(llm.WithTemperature(-1)), llm.ErrBadParameter)
	})
}

func TestChatTools(t *testing.T) {
	t.Run("Tools", func(t *testing.T) {
		tools, err := chatTools([]llm.Tool{weatherTool{name: "get_weather", withSchema: true}, weatherTool{name: "no_input"}})
		require.NoError(t, err)
		require.Len(t, tools, 2)

		// A tool with an input schema has it as its parameters
		assert.Equal(t, "function", tools[0].Type)
		assert.Equal(t, "get_weather", tools[0].Function.Name)
		assert.Equal(t, "Get the weather", tools[0].Function.Description)
		var parameters map[string]any
		require.NoError(t, json.Unmarshal(tools[0].Function.Parameters, &parameters))
		assert.Equal(t, "object", parameters["type"])
		assert.Contains(t, parameters["properties"], "city")

		// A tool without an input schema has no parameters
		assert.JSONEq(t, `{"type":"object","properties":{}}`, string(tools[1].Function.Parameters))
	})

	t.Run("Request", func(t *testing.T) {
		// The tools are sent in the request
		tools, err := chatTools([]llm.Tool{weatherTool{name: "get_weather", withSchema: true}})
		require.NoError(t, err)
		data, err := json.Marshal(chatRequest{Model: "llama3.2", Tools: tools})
		require.NoError(t, err)
		var request map[string]any
		require.NoError(t, json.Unmarshal(data, &request))
		sent, ok := request["tools"].([]any)
		require.True(t, ok)
		require.Len(t, sent, 1)
		assert.Equal(t, "get_weather", sent[0].(map[string]any)["function"].(map[string]any)["name"])
	})

	t.Run("NoTools", func(t *testing.T) {
		// Without tools, the request has none
		tools, err := chatTools(nil)
		require.NoError(t, err)
		assert.Nil(t, tools)
		data, err := json.Marshal(chatRequest{Model: "llama3.2", Tools: tools})
		require.NoError(t, err)
		assert.NotContains(t, string(data), "tools")
	})

	t.Run("Invalid", func(t *testing.T) {
		for name, tools := range map[string][]llm.Tool{
			"Nil":         {nil},
			"InvalidName": {weatherTool{name: "weatherapi:current"}},
			"Duplicate":   {weatherTool{name: "a"}, weatherTool{name: "a"}},
		} {
			_, err := chatTools(tools)
			assert.ErrorIs(t, err, llm.ErrBadParameter, name)
		}
	})
}

// weatherTool is a tool for tests, with an input schema when withSchema is set
type weatherTool struct {
	name       string
	withSchema bool
}

type weatherInput struct {
	City string `json:"city" help:"City name"`
}

func (t weatherTool) Name() string        { return t.name }
func (t weatherTool) Description() string { return "Get the weather" }
func (t weatherTool) InputSchema() *jsonschema.Schema {
	if !t.withSchema {
		return nil
	}
	return jsonschema.MustFor[weatherInput]()
}
func (t weatherTool) OutputSchema() *jsonschema.Schema { return nil }
func (weatherTool) Timeout() time.Duration             { return 0 }
func (t weatherTool) Run(context.Context, json.RawMessage) (any, error) {
	return map[string]any{"temperature_c": 18}, nil
}
