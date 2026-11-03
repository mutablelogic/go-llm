package llm_test

import (
	"encoding/json"
	"errors"
	"math"
	"testing"
	"time"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

func TestApplyOpts(t *testing.T) {
	t.Run("Empty", func(t *testing.T) {
		opts, err := llm.ApplyOpts()
		require.NoError(t, err)
		assert.False(t, opts.Has("temperature"))
		assert.Nil(t, opts.Get("temperature"))
	})

	t.Run("Set", func(t *testing.T) {
		opts, err := llm.ApplyOpts(llm.WithOpt("temperature", 0.5), nil, llm.WithOpt("max_tokens", uint(100)))
		require.NoError(t, err)
		assert.True(t, opts.Has("temperature"))
		assert.Equal(t, 0.5, opts.Get("temperature"))

		temperature, err := llm.GetOpt[float64](opts, "temperature")
		assert.NoError(t, err)
		assert.Equal(t, 0.5, temperature)

		maxTokens, err := llm.GetOpt[uint](opts, "max_tokens")
		assert.NoError(t, err)
		assert.Equal(t, uint(100), maxTokens)
	})

	t.Run("Replace", func(t *testing.T) {
		opts, err := llm.ApplyOpts(llm.WithOpt("temperature", 0.5), llm.WithOpt("temperature", 0.7))
		require.NoError(t, err)
		assert.Equal(t, 0.7, opts.Get("temperature"))
	})

	t.Run("WrongType", func(t *testing.T) {
		opts, err := llm.ApplyOpts(llm.WithOpt("temperature", "hot"))
		require.NoError(t, err)
		_, err = llm.GetOpt[float64](opts, "temperature")
		assert.ErrorIs(t, err, llm.ErrBadParameter)
		_, err = llm.GetOpt[float64](opts, "missing")
		assert.ErrorIs(t, err, llm.ErrNotFound)
	})

	t.Run("JSON", func(t *testing.T) {
		// A value which is JSON is decoded as the type, unless it's asked for
		// as JSON
		opts, err := llm.ApplyOpts(
			llm.WithOpt("temperature", json.RawMessage(`0.7`)),
			llm.WithOpt("max_tokens", json.RawMessage(`100`)),
			llm.WithOpt("stop_sequences", json.RawMessage(`["END"]`)),
			llm.WithOpt("thinking_effort", json.RawMessage(`"high"`)),
		)
		require.NoError(t, err)
		temperature, err := llm.GetOpt[float64](opts, "temperature")
		assert.NoError(t, err)
		assert.Equal(t, 0.7, temperature)
		maxTokens, err := llm.GetOpt[uint](opts, "max_tokens")
		assert.NoError(t, err)
		assert.Equal(t, uint(100), maxTokens)
		stop, err := llm.GetOpt[[]string](opts, "stop_sequences")
		assert.NoError(t, err)
		assert.Equal(t, []string{"END"}, stop)
		effort, err := llm.GetOpt[string](opts, "thinking_effort")
		assert.NoError(t, err)
		assert.Equal(t, "high", effort)
		raw, err := llm.GetOpt[json.RawMessage](opts, "stop_sequences")
		assert.NoError(t, err)
		assert.JSONEq(t, `["END"]`, string(raw))

		// JSON which is null isn't of any type, unless it's asked for as JSON
		opts, err = llm.ApplyOpts(llm.WithOpt("temperature", json.RawMessage(` null `)), llm.WithOpt("thinking", json.RawMessage(`null`)))
		require.NoError(t, err)
		_, err = llm.GetOpt[float64](opts, "temperature")
		assert.ErrorIs(t, err, llm.ErrBadParameter)
		_, err = llm.GetOpt[bool](opts, "thinking")
		assert.ErrorIs(t, err, llm.ErrBadParameter)
		raw, err = llm.GetOpt[json.RawMessage](opts, "thinking")
		assert.NoError(t, err)
		assert.Equal(t, `null`, string(raw))

		// JSON which isn't of the type
		opts, err = llm.ApplyOpts(llm.WithOpt("temperature", json.RawMessage(`0.7`)), llm.WithOpt("thinking_effort", json.RawMessage(`"high"`)))
		require.NoError(t, err)
		_, err = llm.GetOpt[uint](opts, "temperature")
		assert.ErrorIs(t, err, llm.ErrBadParameter)
		_, err = llm.GetOpt[float64](opts, "thinking_effort")
		assert.ErrorIs(t, err, llm.ErrBadParameter)
	})

	t.Run("Delete", func(t *testing.T) {
		opts, err := llm.ApplyOpts(llm.WithOpt("temperature", 0.5), func(o *llm.Opts) error {
			o.Delete("temperature")
			o.Delete("missing")
			return nil
		})
		require.NoError(t, err)
		assert.False(t, opts.Has("temperature"))
		assert.Empty(t, opts.Keys())
	})

	t.Run("Opt", func(t *testing.T) {
		// The options are applied once, and passed on as their values
		calls := 0
		opts, err := llm.ApplyOpts(llm.WithOpt("temperature", 0.5), func(o *llm.Opts) error {
			calls++
			return nil
		})
		require.NoError(t, err)
		opts.Delete("missing")
		copied, err := llm.ApplyOpts(opts.Opt())
		require.NoError(t, err)
		assert.Equal(t, 1, calls)
		assert.Equal(t, 0.5, copied.Get("temperature"))

		// Changing the options afterwards doesn't change the copy
		opts.Delete("temperature")
		assert.Equal(t, 0.5, copied.Get("temperature"))
	})

	t.Run("Error", func(t *testing.T) {
		optErr := errors.New("bad option")
		_, err := llm.ApplyOpts(llm.WithOpt("temperature", 0.5), func(*llm.Opts) error { return optErr })
		assert.ErrorIs(t, err, optErr)
	})
}

func TestWithOpts(t *testing.T) {
	t.Run("Valid", func(t *testing.T) {
		for _, test := range []struct {
			opt   llm.Opt
			key   string
			value any
		}{
			{llm.WithTemperature(0.7), llm.TemperatureKey, 0.7},
			{llm.WithTopP(0.9), llm.TopPKey, 0.9},
			{llm.WithTopK(40), llm.TopKKey, uint(40)},
			{llm.WithMaxTokens(100), llm.MaxTokensKey, uint(100)},
			{llm.WithStopSequences("END", "STOP"), llm.StopSequencesKey, []string{"END", "STOP"}},
			{llm.WithSeed(42), llm.SeedKey, uint(42)},
			{llm.WithPresencePenalty(-0.5), llm.PresencePenaltyKey, -0.5},
			{llm.WithFrequencyPenalty(0.5), llm.FrequencyPenaltyKey, 0.5},
			{llm.WithSystemPrompt("Be brief"), llm.SystemPromptKey, "Be brief"},
			{llm.WithThinking(true), llm.ThinkingKey, true},
			{llm.WithThinkingEffort("high"), llm.ThinkingEffortKey, "high"},
			{llm.WithJSONSchema(json.RawMessage(`{"type":"object"}`)), llm.JSONSchemaKey, json.RawMessage(`{"type":"object"}`)},
			{llm.WithToolChoice("any"), llm.ToolChoiceKey, "any"},
			{llm.WithContextWindow(8192), llm.ContextWindowKey, uint(8192)},
			{llm.WithKeepAlive(-1), llm.KeepAliveKey, time.Duration(-1)},
			{llm.WithUser("user_1"), llm.UserKey, "user_1"},
			{llm.WithServiceTier("auto"), llm.ServiceTierKey, "auto"},
			{llm.WithTaskType("RETRIEVAL_DOCUMENT"), llm.TaskTypeKey, "RETRIEVAL_DOCUMENT"},
			{llm.WithTitle("Notes"), llm.TitleKey, "Notes"},
			{llm.WithDimensions(256), llm.DimensionsKey, uint(256)},
		} {
			opts, err := llm.ApplyOpts(test.opt)
			require.NoError(t, err, test.key)
			assert.Equal(t, test.value, opts.Get(test.key), test.key)
		}
	})

	t.Run("Invalid", func(t *testing.T) {
		for name, opt := range map[string]llm.Opt{
			"NegativeTemperature": llm.WithTemperature(-0.1),
			"NaNTemperature":      llm.WithTemperature(math.NaN()),
			"InfTemperature":      llm.WithTemperature(math.Inf(1)),
			"TopPAboveOne":        llm.WithTopP(1.1),
			"ZeroTopK":            llm.WithTopK(0),
			"ZeroMaxTokens":       llm.WithMaxTokens(0),
			"EmptyStopSequence":   llm.WithStopSequences("END", ""),
			"InfPresencePenalty":  llm.WithPresencePenalty(math.Inf(1)),
			"NaNFrequencyPenalty": llm.WithFrequencyPenalty(math.NaN()),
			"UnknownEffort":       llm.WithThinkingEffort("max"),
			"InvalidJSONSchema":   llm.WithJSONSchema(json.RawMessage(`{`)),
			"UnknownToolChoice":   llm.WithToolChoice("required"),
			"ZeroContextWindow":   llm.WithContextWindow(0),
			"EmptyUser":           llm.WithUser(""),
			"EmptyServiceTier":    llm.WithServiceTier(""),
			"EmptyTaskType":       llm.WithTaskType(""),
			"ZeroDimensions":      llm.WithDimensions(0),
		} {
			_, err := llm.ApplyOpts(opt)
			assert.ErrorIs(t, err, llm.ErrBadParameter, name)
		}
	})

	t.Run("StopSequencesCopied", func(t *testing.T) {
		values := []string{"END"}
		opts, err := llm.ApplyOpts(llm.WithStopSequences(values...))
		require.NoError(t, err)
		values[0] = "CHANGED"
		assert.Equal(t, []string{"END"}, opts.Get(llm.StopSequencesKey))
	})
}
