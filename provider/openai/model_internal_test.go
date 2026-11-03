package openai

import (
	"encoding/json"
	"strings"
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
	yes, no := true, false

	// model returns a model with the ID and fields, from a client for the
	// endpoint
	model := func(t *testing.T, endpoint, id, capabilities string) *Model {
		t.Helper()
		c, err := New(endpoint, "")
		require.NoError(t, err)
		var fields map[string]json.RawMessage
		if capabilities != "" {
			fields = map[string]json.RawMessage{"capabilities": json.RawMessage(capabilities)}
		}
		return &Model{provider: c, data: apiModel{ID: id, Fields: fields}}
	}

	t.Run("Reported", func(t *testing.T) {
		// A model has the capabilities the server returns, as Mistral does
		m := model(t, MistralEndpoint, "mistral-ocr-latest", `{"completion_chat":false,"function_calling":true,"reasoning":false,"vision":true,"ocr":true,"audio_transcription":false,"moderation":false}`)
		assert.Equal(t, llm.ModelCap{Completion: &no, Tools: &yes, Thinking: &no, Vision: &yes, OCR: &yes, Transcription: &no}, m.Capabilities())
		m = model(t, MistralEndpoint, "mistral-small-latest", `{"completion_chat":true,"reasoning":true}`)
		assert.Equal(t, llm.ModelCap{Completion: &yes, Thinking: &yes}, m.Capabilities())

		// Or none, when it returns none, or they aren't capabilities
		assert.Equal(t, llm.ModelCap{}, model(t, "http://localhost:8000", "gpt-4o", "").Capabilities())
		assert.Equal(t, llm.ModelCap{}, model(t, "http://localhost:8000", "x", `["vision"]`).Capabilities())
	})

	t.Run("Mistral", func(t *testing.T) {
		// Mistral's embedding models embed, which it doesn't report, but
		// models with the same names on another server don't
		for _, id := range []string{"mistral-embed", "mistral-embed-2312", "codestral-embed-2505"} {
			m := model(t, MistralEndpoint, id, `{"completion_chat":false,"reasoning":false}`)
			assert.Equal(t, llm.ModelCap{Completion: &no, Thinking: &no, Embedding: &yes}, m.Capabilities(), id)
		}
		assert.Equal(t, llm.ModelCap{}, model(t, "http://localhost:8000", "mistral-embed", "").Capabilities())
	})

	t.Run("OpenAI", func(t *testing.T) {
		// OpenAI's models have the capabilities known by their names, as
		// OpenAI reports none, of which those which reply with Chat
		// Completions complete
		for id, want := range map[string]string{
			"chat-latest":                       "completion, tools, vision",
			"gpt-4.1":                           "completion, tools, vision",
			"gpt-4.1-mini-2025-04-14":           "completion, tools, vision",
			"gpt-4o":                            "completion, tools, vision",
			"gpt-4o-mini-2024-07-18":            "completion, tools, vision",
			"gpt-5":                             "completion, tools, thinking, vision",
			"gpt-5-nano-2025-08-07":             "completion, tools, thinking, vision",
			"gpt-5.1-2025-11-13":                "completion, tools, thinking, vision",
			"gpt-5.4-mini-2026-03-17":           "completion, tools, thinking, vision",
			"gpt-5.5":                           "completion, tools, thinking, vision",
			"gpt-5.6-luna":                      "completion, tools, thinking, vision",
			"gpt-6-sol":                         "completion, tools, thinking, vision",
			"gpt-6.1-terra":                     "completion, tools, thinking, vision",
			"o3-2025-04-16":                     "completion, tools, thinking, vision",
			"gpt-6-astra":                       "completion, thinking, vision",
			"gpt-6.1-sol":                       "completion, thinking, vision",
			"gpt-5-search-api-2025-10-14":       "completion, vision",
			"gpt-5-pro":                         "thinking, vision",
			"gpt-5.5-pro-2026-04-23":            "thinking, vision",
			"gpt-5.3-codex":                     "thinking, vision",
			"gpt-3.5-turbo-16k":                 "",
			"gpt-3.5-turbo-instruct-0914":       "",
			"gpt-4o-search-preview":             "",
			"gpt-4o-mini-search-preview":        "",
			"whisper-1":                         "transcription, translation",
			"gpt-4o-transcribe":                 "transcription",
			"gpt-4o-mini-transcribe-2025-12-15": "transcription",
			"gpt-4o-transcribe-diarize":         "transcription",
			"gpt-transcribe":                    "transcription",
			"gpt-live-transcribe":               "transcription",
			"gpt-realtime-whisper":              "transcription",
			"gpt-realtime-translate":            "translation",
			"tts-1-hd-1106":                     "",
			"gpt-4o-mini-tts-2025-12-15":        "",
			"gpt-audio-mini-2025-12-15":         "",
			"gpt-realtime-2.1-mini":             "",
			"gpt-live-1":                        "",
			"gpt-image-2.5-flare-2026-09-08":    "vision",
			"chatgpt-image-latest":              "vision",
			"omni-moderation-latest":            "vision",
			"text-embedding-3-small":            "embedding",
			"text-embedding-ada-002":            "embedding",
		} {
			capabilities := model(t, OpenAIEndpoint, id, "").Capabilities()
			assert.Equal(t, want, capabilities.String(), id)
			if !strings.Contains(want, "completion") {
				assert.Equal(t, &no, capabilities.Completion, id)
			}
		}

		// A model which isn't known has none, as do the GPT-3.5 Turbo
		// models which aren't known to be deprecated, and a known model on
		// another server
		assert.Equal(t, llm.ModelCap{}, model(t, OpenAIEndpoint, "gpt-7", "").Capabilities())
		assert.Equal(t, llm.ModelCap{}, model(t, OpenAIEndpoint, "gpt-3.5-turbo", "").Capabilities())
		assert.Equal(t, llm.ModelCap{}, model(t, OpenAIEndpoint, "gpt-3.5-turbo-0125", "").Capabilities())
		assert.Equal(t, llm.ModelCap{}, model(t, "http://localhost:8000", "gpt-4o", "").Capabilities())
	})

	t.Run("Copies", func(t *testing.T) {
		// A model's capabilities can be changed without changing another's
		capabilities := model(t, OpenAIEndpoint, "gpt-4o", "").Capabilities()
		*capabilities.Completion = false
		assert.Equal(t, &yes, model(t, OpenAIEndpoint, "gpt-4o", "").Capabilities().Completion)
	})
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
