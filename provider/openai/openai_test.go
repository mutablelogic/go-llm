package openai_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	openai "github.com/mutablelogic/go-llm/provider/openai"
	schema "github.com/mutablelogic/go-llm/provider/schema"
	toolkit "github.com/mutablelogic/go-llm/toolkit"
	jsonschema "github.com/mutablelogic/go-server/pkg/jsonschema"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

// The tests in this file make requests to OpenAI's API, so they're skipped
// unless OPENAI_API_KEY is set, and with -short. They use cheap models, which
// OPENAI_MODEL and OPENAI_EMBEDDING_MODEL can change. As replies vary, they
// check what replies are like, rather than what they say.

const (
	liveModel          = "gpt-4o-mini"
	liveEmbeddingModel = "text-embedding-3-small"
	liveTimeout        = time.Minute
)

// liveClient returns a client of OpenAI's API, with the key in
// OPENAI_API_KEY, or skips the test when it isn't set, or with -short
func liveClient(t *testing.T) *openai.Client {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping requests to OpenAI's API with -short")
	}
	key := os.Getenv("OPENAI_API_KEY")
	if key == "" {
		t.Skip("OPENAI_API_KEY isn't set")
	}
	c, err := openai.New("", key)
	require.NoError(t, err)
	return c
}

// liveModelNamed returns the model of the client named in the environment
// variable, or else the name
func liveModelNamed(t *testing.T, ctx context.Context, c *openai.Client, env, name string) llm.Model {
	t.Helper()
	if value := os.Getenv(env); value != "" {
		name = value
	}
	model, err := c.GetModel(ctx, name)
	require.NoError(t, err, name)
	return model
}

// liveUser returns a message from the user with the text
func liveUser(t *testing.T, text string) llm.Message {
	t.Helper()
	message, err := schema.NewMessage("user", text)
	require.NoError(t, err)
	return message
}

// weatherTool is a tool which the model is asked to call, which isn't run
type weatherTool struct{ toolkit.Base }

var _ llm.Tool = weatherTool{}

func (weatherTool) Name() string        { return "get_weather" }
func (weatherTool) Description() string { return "Returns the current weather in a city" }
func (weatherTool) InputSchema() *jsonschema.Schema {
	return jsonschema.MustFor[struct {
		City string `json:"city" jsonschema:"The city, such as Berlin"`
	}]()
}
func (weatherTool) Run(context.Context, json.RawMessage) (any, error) { return nil, nil }

func Test_Live(t *testing.T) {
	c := liveClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), liveTimeout)
	defer cancel()

	t.Run("Ping", func(t *testing.T) {
		assert.NoError(t, c.Ping(ctx))
	})

	t.Run("Models", func(t *testing.T) {
		// The models include the chat and embedding models
		models, err := c.ListModels(ctx)
		require.NoError(t, err)
		assert.NotEmpty(t, models)
		chat := liveModelNamed(t, ctx, c, "OPENAI_MODEL", liveModel)
		assert.Equal(t, c, chat.Provider())
		liveModelNamed(t, ctx, c, "OPENAI_EMBEDDING_MODEL", liveEmbeddingModel)
	})

	t.Run("Reply", func(t *testing.T) {
		// The model replies, which uses tokens, and it stops when it's done
		model := liveModelNamed(t, ctx, c, "OPENAI_MODEL", liveModel)
		reply, usage, err := c.WithoutSession(ctx, model, []llm.Message{liveUser(t, "Reply with the single word: pong")}, nil, nil, llm.WithMaxTokens(16))
		require.NoError(t, err)
		assert.Equal(t, "assistant", reply.Role())
		assert.Contains(t, strings.ToLower(reply.Text()), "pong")
		assert.Equal(t, schema.ResultStop, reply.(*schema.Message).Result)
		assert.NotZero(t, usage.InputTokens)
		assert.NotZero(t, usage.OutputTokens)
	})

	t.Run("MaxTokens", func(t *testing.T) {
		// A reply which reaches the most tokens is truncated
		model := liveModelNamed(t, ctx, c, "OPENAI_MODEL", liveModel)
		reply, _, err := c.WithoutSession(ctx, model, []llm.Message{liveUser(t, "Count from one to one hundred in words")}, nil, nil, llm.WithMaxTokens(5))
		require.NoError(t, err)
		assert.Equal(t, schema.ResultMaxTokens, reply.(*schema.Message).Result)
	})

	t.Run("Stream", func(t *testing.T) {
		// The reply is streamed in pieces, which make its text
		model := liveModelNamed(t, ctx, c, "OPENAI_MODEL", liveModel)
		var text strings.Builder
		pieces := 0
		reply, usage, err := c.WithoutSession(ctx, model, []llm.Message{liveUser(t, "Say hello in five words")}, nil, func(kind llm.StreamType, piece string) {
			if kind == llm.StreamText {
				text.WriteString(piece)
				pieces++
			}
		}, llm.WithMaxTokens(32))
		require.NoError(t, err)
		assert.Greater(t, pieces, 1)
		assert.Equal(t, reply.Text(), text.String())
		assert.NotZero(t, usage.OutputTokens)
	})

	t.Run("ToolCall", func(t *testing.T) {
		// The model calls the tool, with its input as JSON
		model := liveModelNamed(t, ctx, c, "OPENAI_MODEL", liveModel)
		reply, _, err := c.WithoutSession(ctx, model, []llm.Message{liveUser(t, "What's the weather in Berlin?")}, []llm.Tool{weatherTool{}}, nil, llm.WithToolChoice("any"))
		require.NoError(t, err)
		assert.Equal(t, schema.ResultToolCall, reply.(*schema.Message).Result)
		calls := reply.ToolCalls()
		if assert.Len(t, calls, 1) {
			assert.Equal(t, "get_weather", calls[0].Name)
			assert.NotEmpty(t, calls[0].ID)
			var input struct {
				City string `json:"city"`
			}
			require.NoError(t, json.Unmarshal(calls[0].Input, &input))
			assert.Contains(t, input.City, "Berlin")
		}
	})

	t.Run("Embed", func(t *testing.T) {
		// Each message is a vector, of the model's dimensions, or as many as
		// are asked for
		model := liveModelNamed(t, ctx, c, "OPENAI_EMBEDDING_MODEL", liveEmbeddingModel)
		messages := []llm.Message{liveUser(t, "The cat sat on the mat"), liveUser(t, "A dog barked")}
		vectors, usage, err := c.Embed(ctx, model, messages)
		require.NoError(t, err)
		if assert.Len(t, vectors, 2) {
			assert.NotEmpty(t, vectors[0])
			assert.Len(t, vectors[1], len(vectors[0]))
		}
		assert.NotZero(t, usage.InputTokens)

		vectors, _, err = c.Embed(ctx, model, messages[:1], llm.WithDimensions(256))
		require.NoError(t, err)
		if assert.Len(t, vectors, 1) {
			assert.Len(t, vectors[0], 256)
		}
	})
}

func Test_LiveTranscribe(t *testing.T) {
	c := liveClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), liveTimeout)
	defer cancel()
	speech := func(name string) llm.Resource {
		data, err := os.ReadFile(filepath.Join("..", "..", "testdata", name))
		require.NoError(t, err)
		return &schema.Resource{ResourceMeta: schema.ResourceMeta{ContentType: schema.ResourceType(data, name), Data: data}}
	}

	t.Run("Whisper", func(t *testing.T) {
		// The text, its language, which whisper detects, its segments, and
		// the seconds
		model := liveModelNamed(t, ctx, c, "OPENAI_WHISPER_MODEL", "whisper-1")
		transcript, usage, err := c.Transcribe(ctx, model, speech("speech-en.m4a"), nil, llm.WithSegments(true))
		require.NoError(t, err)
		t.Logf("%+v, %+v", transcript, usage)
		assert.Contains(t, strings.ToLower(transcript.Text), "quick brown fox")
		assert.Equal(t, "en", transcript.Language)
		if assert.NotEmpty(t, transcript.Segments) {
			last := transcript.Segments[len(transcript.Segments)-1]
			assert.Greater(t, last.End, last.Start)
		}
		assert.Positive(t, usage.Seconds)
	})

	t.Run("Translate", func(t *testing.T) {
		// French speech, as English text
		model := liveModelNamed(t, ctx, c, "OPENAI_WHISPER_MODEL", "whisper-1")
		transcript, _, err := c.Translate(ctx, model, speech("speech-fr.wav"), nil)
		require.NoError(t, err)
		t.Logf("%+v", transcript)
		assert.Contains(t, strings.ToLower(transcript.Text), "hello")
		assert.Equal(t, "en", transcript.Language)
	})

	t.Run("Transcribe", func(t *testing.T) {
		// A model which returns text, and the tokens used
		model := liveModelNamed(t, ctx, c, "OPENAI_TRANSCRIBE_MODEL", "gpt-4o-mini-transcribe")
		transcript, usage, err := c.Transcribe(ctx, model, speech("speech-en.m4a"), nil)
		require.NoError(t, err)
		t.Logf("%+v, %+v", transcript, usage)
		assert.Contains(t, strings.ToLower(transcript.Text), "quick brown fox")
		assert.Empty(t, transcript.Segments)
		assert.Positive(t, usage.InputTokens)
	})

	t.Run("Stream", func(t *testing.T) {
		// The pieces of the text, as they're transcribed, which are the
		// transcript's
		model := liveModelNamed(t, ctx, c, "OPENAI_TRANSCRIBE_MODEL", "gpt-4o-mini-transcribe")
		var pieces []string
		transcript, usage, err := c.Transcribe(ctx, model, speech("speech-en.m4a"), func(text string, segment *llm.TranscriptSegment) {
			pieces = append(pieces, text)
		})
		require.NoError(t, err)
		t.Logf("%d pieces, %+v", len(pieces), usage)
		assert.Greater(t, len(pieces), 1)
		assert.Equal(t, transcript.Text, strings.TrimSpace(strings.Join(pieces, "")))
		assert.Positive(t, usage.InputTokens)
	})

	t.Run("Speakers", func(t *testing.T) {
		// A diarizing model, whose segments have their speakers
		model := liveModelNamed(t, ctx, c, "OPENAI_DIARIZE_MODEL", "gpt-4o-transcribe-diarize")
		transcript, _, err := c.Transcribe(ctx, model, speech("speech-en.m4a"), nil, llm.WithSpeakers(true))
		require.NoError(t, err)
		t.Logf("%+v", transcript)
		if assert.NotEmpty(t, transcript.Segments) {
			assert.NotEmpty(t, transcript.Segments[0].Speaker)
		}
	})
}
