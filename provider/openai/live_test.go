package openai_test

import (
	"context"
	"encoding/json"
	"os"
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
