package anthropic_test

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	anthropic "github.com/mutablelogic/go-llm/provider/anthropic"
	schema "github.com/mutablelogic/go-llm/provider/schema"
	jsonschema "github.com/mutablelogic/go-server/pkg/jsonschema"
	types "github.com/mutablelogic/go-server/pkg/types"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

// liveModel is the model the live tests use, which is the cheapest
const liveModel = "claude-haiku-5-5"

// liveClient returns a client of Anthropic's API, with the key in
// ANTHROPIC_API_KEY, and its model, or skips the test, when there's no key,
// or the tests are short, as its requests are paid for
func liveClient(t *testing.T) (*anthropic.Client, llm.Model) {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping a test of Anthropic's API in short mode")
	}
	key := os.Getenv("ANTHROPIC_API_KEY")
	if key == "" {
		t.Skip("ANTHROPIC_API_KEY not set")
	}
	c, err := anthropic.New("", key)
	require.NoError(t, err)
	model, err := c.GetModel(context.Background(), liveModel)
	require.NoError(t, err)
	return c, model
}

// cityTool is a tool which returns the weather in a city
type cityTool struct{}

func (cityTool) Name() string        { return "weather" }
func (cityTool) Description() string { return "Returns the weather in a city" }
func (cityTool) InputSchema() *jsonschema.Schema {
	return jsonschema.MustFor[struct {
		City string `json:"city" help:"Name of the city"`
	}]()
}
func (cityTool) OutputSchema() *jsonschema.Schema { return nil }
func (cityTool) Run(context.Context, json.RawMessage) (any, error) {
	return map[string]any{"weather": "sunny", "celsius": 21}, nil
}

func Test_Live(t *testing.T) {
	ctx := context.Background()
	c, model := liveClient(t)

	t.Run("Models", func(t *testing.T) {
		models, err := c.ListModels(ctx)
		require.NoError(t, err)
		assert.NotEmpty(t, models)
		assert.NotNil(t, model.InputTokenLimit())
		assert.True(t, types.Value(model.Capabilities().Chat))
	})

	t.Run("Reply", func(t *testing.T) {
		var streamed strings.Builder
		reply, usage, err := c.WithoutSession(ctx, model, []llm.Message{
			&schema.Message{MessageMeta: schema.MessageMeta{Sender: "user", Content: []schema.ContentBlock{{Text: types.Ptr("Reply with only the word: pong")}}}},
		}, nil, func(kind llm.StreamType, text string) {
			if kind == llm.StreamText {
				streamed.WriteString(text)
			}
		}, llm.WithOpt(llm.ThinkingKey, false), llm.WithOpt(llm.MaxTokensKey, uint(200)))
		require.NoError(t, err)
		assert.Contains(t, strings.ToLower(reply.Text()), "pong")
		assert.Equal(t, reply.Text(), streamed.String())
		assert.NotZero(t, usage.InputTokens)
		assert.NotZero(t, usage.OutputTokens)
	})

	t.Run("Tools", func(t *testing.T) {
		// The model calls the tool, and replies with its result
		messages := []llm.Message{
			&schema.Message{MessageMeta: schema.MessageMeta{Sender: "user", Content: []schema.ContentBlock{{Text: types.Ptr("What's the weather in Berlin? Use the tool.")}}}},
		}
		tools := []llm.Tool{cityTool{}}
		reply, _, err := c.WithoutSession(ctx, model, messages, tools, nil, llm.WithOpt(llm.MaxTokensKey, uint(2000)))
		require.NoError(t, err)
		require.NotEmpty(t, reply.ToolCalls(), "it calls the tool")
		call := reply.ToolCalls()[0]
		assert.Equal(t, "weather", call.Name)
		assert.Contains(t, string(call.Input), "Berlin")

		result, _ := json.Marshal(map[string]any{"weather": "sunny", "celsius": 21})
		messages = append(messages, reply, &schema.Message{MessageMeta: schema.MessageMeta{Sender: "tool", Content: []schema.ContentBlock{
			{ToolResult: &schema.ToolResult{ID: call.ID, Name: call.Name, Content: result}},
		}}})
		reply, _, err = c.WithoutSession(ctx, model, messages, tools, nil, llm.WithOpt(llm.MaxTokensKey, uint(2000)))
		require.NoError(t, err)
		assert.Empty(t, reply.ToolCalls())
		assert.Contains(t, reply.Text(), "21")
	})

	t.Run("Measure", func(t *testing.T) {
		tokens, err := c.Measure(ctx, model, "The quick brown fox jumps over the lazy dog.")
		require.NoError(t, err)
		assert.Greater(t, tokens, uint(5))
		assert.Less(t, tokens, uint(20))
	})
}
