package openai

import (
	"encoding/json"
	"testing"
	"time"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

func TestOpenAIModelLimits(t *testing.T) {
	// model returns a model with the ID, from a client for the endpoint
	model := func(t *testing.T, endpoint, id string, fields map[string]json.RawMessage) *Model {
		t.Helper()
		c, err := New(endpoint, "")
		require.NoError(t, err)
		return &Model{provider: c, data: apiModel{ID: id, Fields: fields}}
	}

	// limits returns the input and output token limits of the model, or zero
	limits := func(m *Model) [2]uint {
		var result [2]uint
		if limit := m.InputTokenLimit(); limit != nil {
			result[0] = *limit
		}
		if limit := m.OutputTokenLimit(); limit != nil {
			result[1] = *limit
		}
		return result
	}

	t.Run("Table", func(t *testing.T) {
		// OpenAI's models, and their dated versions, use their limits
		for id, want := range map[string][2]uint{
			"gpt-4o":             {128000, 16384},
			"gpt-4o-2024-08-06":  {128000, 16384},
			"gpt-4o-mini":        {128000, 16384},
			"gpt-3.5-turbo-0125": {16385, 4096},
			"o1":                 {200000, 100000},
			"o1-mini-2024-09-12": {128000, 65536},
			"gpt-5-mini":         {400000, 128000},
		} {
			assert.Equal(t, want, limits(model(t, OpenAIEndpoint, id, nil)), "model %q", id)
		}
	})

	t.Run("NotInTable", func(t *testing.T) {
		// Other models, and newer models, have no limits, rather than the
		// limits of a model their name starts with
		for _, id := range []string{
			"gpt-4o-transcribe", "gpt-4o-mini-tts", "gpt-4o-search-preview-2025-03-11",
			"gpt-5.4", "gpt-5-codex", "o1-pro", "gpt-3.5-turbo-instruct", "gpt-3.5-turbo-16k",
			"text-embedding-3-large",
		} {
			assert.Equal(t, [2]uint{0, 0}, limits(model(t, OpenAIEndpoint, id, nil)), "model %q", id)
		}
	})

	t.Run("OtherServers", func(t *testing.T) {
		// A model with the same name on another server doesn't use the table
		assert.Equal(t, [2]uint{0, 0}, limits(model(t, "http://localhost:8000", "gpt-4o", nil)))
		assert.Equal(t, [2]uint{0, 0}, limits(model(t, DeepSeekEndpoint, "gpt-4o", nil)))
	})

	t.Run("Fields", func(t *testing.T) {
		// Limits the server returns are used before the table
		m := model(t, OpenAIEndpoint, "gpt-4o", map[string]json.RawMessage{
			"context_length":    json.RawMessage(`1000`),
			"max_output_tokens": json.RawMessage(`100`),
		})
		assert.Equal(t, [2]uint{1000, 100}, limits(m))

		// Including Mistral's context length
		m = model(t, MistralEndpoint, "mistral-small-2603", map[string]json.RawMessage{
			"max_context_length": json.RawMessage(`256000`),
		})
		assert.Equal(t, [2]uint{256000, 0}, limits(m))
	})
}

func TestCapabilities(t *testing.T) {
	// A model has the capabilities the server returns, as Mistral does
	yes, no := true, false
	m := &Model{data: apiModel{ID: "mistral-ocr-latest", Fields: map[string]json.RawMessage{
		"capabilities": json.RawMessage(`{"completion_chat":false,"function_calling":true,"reasoning":false,"vision":true,"ocr":true,"audio_transcription":false,"moderation":false}`),
	}}}
	assert.Equal(t, llm.ModelCap{Completion: &no, Tools: &yes, Thinking: &no, Vision: &yes, OCR: &yes, Transcription: &no}, m.Capabilities())

	// A model which reasons
	m = &Model{data: apiModel{ID: "mistral-small-latest", Fields: map[string]json.RawMessage{
		"capabilities": json.RawMessage(`{"completion_chat":true,"reasoning":true}`),
	}}}
	assert.Equal(t, llm.ModelCap{Completion: &yes, Thinking: &yes}, m.Capabilities())

	// And those which are known by the model's name, as Mistral's embedding
	// models embed
	for _, id := range []string{"mistral-embed", "mistral-embed-2312", "codestral-embed-2505"} {
		m = &Model{data: apiModel{ID: id, Fields: map[string]json.RawMessage{
			"capabilities": json.RawMessage(`{"completion_chat":false,"reasoning":false}`),
		}}}
		assert.Equal(t, llm.ModelCap{Completion: &no, Thinking: &no, Embedding: &yes}, m.Capabilities(), id)
	}

	// Or none, as OpenAI returns none
	assert.Equal(t, llm.ModelCap{}, (&Model{data: apiModel{ID: "gpt-4o"}}).Capabilities())
	assert.Equal(t, llm.ModelCap{}, (&Model{data: apiModel{ID: "x", Fields: map[string]json.RawMessage{"capabilities": json.RawMessage(`["vision"]`)}}}).Capabilities())
}

func TestAliases(t *testing.T) {
	// A model has the aliases the server returns, as Mistral does, without
	// its own name, or none
	m := &Model{data: apiModel{ID: "mistral-small-2603", Fields: map[string]json.RawMessage{
		"aliases": json.RawMessage(`["mistral-small-latest","","mistral-small-2603"]`),
	}}}
	assert.Equal(t, []string{"mistral-small-latest"}, m.Aliases())
	assert.Empty(t, (&Model{data: apiModel{ID: "gpt-4o"}}).Aliases())
	assert.Empty(t, (&Model{data: apiModel{ID: "gpt-4o", Fields: map[string]json.RawMessage{"aliases": json.RawMessage(`"x"`)}}}).Aliases())
}

func TestShutDown(t *testing.T) {
	now := time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC)
	for date, want := range map[string]bool{
		`"2026-10-09"`: true,
		`"2026-10-10"`: true, // shut down on the day
		`"2026-10-11"`: false,
		`null`:         false,
		`""`:           false,
		`"soon"`:       false,
		`42`:           false,
	} {
		model := apiModel{ID: "model", Fields: map[string]json.RawMessage{"shutdown_date": json.RawMessage(date)}}
		assert.Equal(t, want, model.shutDown(now), "shutdown_date %s", date)
	}

	// A model without a date hasn't been shut down
	assert.False(t, apiModel{ID: "model"}.shutDown(now))
}
