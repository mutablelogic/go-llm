package gemini_test

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	gemini "github.com/mutablelogic/go-llm/provider/gemini"
	schema "github.com/mutablelogic/go-llm/provider/schema"
	jsonschema "github.com/mutablelogic/go-server/pkg/jsonschema"
	types "github.com/mutablelogic/go-server/pkg/types"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

// Test_Live lists Gemini's models, gets one which replies, and measures
// text, with the key in GEMINI_API_KEY, which isn't paid for, or skips,
// when there's no key, or the tests are short
func Test_Live(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping a test of the Gemini API in short mode")
	}
	key := os.Getenv("GEMINI_API_KEY")
	if key == "" {
		t.Skip("GEMINI_API_KEY not set")
	}
	ctx := context.Background()
	c, err := gemini.New("", key)
	require.NoError(t, err)
	require.NoError(t, c.Ping(ctx))

	// A model which replies, which is got again, by its name
	models, err := c.ListModels(ctx)
	require.NoError(t, err)
	require.NotEmpty(t, models)
	var name string
	for _, model := range models {
		if types.Value(model.Capabilities().Completion) && model.InputTokenLimit() != nil {
			name = model.Name()
			break
		}
	}
	require.NotEmpty(t, name, "a model which replies")
	model, err := c.GetModel(ctx, name)
	require.NoError(t, err)
	assert.Equal(t, name, model.Name())

	// Text is measured, in tokens
	tokens, err := c.Measure(ctx, model, "The quick brown fox jumps over the lazy dog.")
	require.NoError(t, err)
	assert.Greater(t, tokens, uint(5))
	assert.Less(t, tokens, uint(20))
}

// Test_LiveEmbed embeds two texts with gemini-embedding-2, with the key in
// GEMINI_API_KEY, which may be paid for, or skips, when there's no key, or
// the tests are short
func Test_LiveEmbed(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping a test of the Gemini API in short mode")
	}
	key := os.Getenv("GEMINI_API_KEY")
	if key == "" {
		t.Skip("GEMINI_API_KEY not set")
	}
	ctx := context.Background()
	c, err := gemini.New("", key)
	require.NoError(t, err)
	model, err := c.GetModel(ctx, "gemini-embedding-2")
	require.NoError(t, err)
	var messages []llm.Message
	for _, text := range []string{"A cat sat on the mat.", "The quarterly report is due on Friday."} {
		message, err := schema.NewMessage("user", text)
		require.NoError(t, err)
		messages = append(messages, message)
	}
	vectors, _, err := c.Embed(ctx, model, messages, llm.WithTaskType("semantic_similarity"), llm.WithDimensions(256))
	require.NoError(t, err)
	require.Len(t, vectors, 2)
	assert.Len(t, vectors[0], 256)
	assert.Len(t, vectors[1], 256)
	assert.NotEqual(t, vectors[0], vectors[1])
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

// Test_LiveChat asks a Gemini 3 model about the weather, which it calls a
// tool for, whose call, with its signature, and result, are sent back, for
// its reply, with the key in GEMINI_API_KEY, which is paid for, or skips,
// when there's no key, or the tests are short
func Test_LiveChat(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping a test of the Gemini API in short mode")
	}
	key := os.Getenv("GEMINI_API_KEY")
	if key == "" {
		t.Skip("GEMINI_API_KEY not set")
	}
	ctx := context.Background()
	c, err := gemini.New("", key)
	require.NoError(t, err)
	model, err := c.GetModel(ctx, "gemini-3.5-flash-lite")
	require.NoError(t, err)
	question, err := schema.NewMessage("user", "What's the weather in Berlin? Use the tool.")
	require.NoError(t, err)

	// The model calls the tool
	tools := []llm.Tool{cityTool{}}
	reply, _, err := c.WithoutSession(ctx, model, []llm.Message{question}, tools, nil)
	require.NoError(t, err)
	calls := reply.ToolCalls()
	require.Len(t, calls, 1, "it calls the tool")
	assert.Equal(t, "weather", calls[0].Name)

	// Its result is sent back, with the call, and its signature
	result := &schema.Message{MessageMeta: schema.MessageMeta{Sender: "tool", Content: []schema.ContentBlock{
		{ToolResult: &schema.ToolResult{ID: calls[0].ID, Name: "weather", Content: json.RawMessage(`{"weather":"sunny","celsius":21}`)}},
	}}}
	var streamed int
	final, usage, err := c.WithoutSession(ctx, model, []llm.Message{question, reply, result}, tools, func(llm.StreamType, string) { streamed++ })
	require.NoError(t, err)
	assert.Contains(t, final.Text(), "21")
	assert.Positive(t, streamed)
	assert.Positive(t, usage.InputTokens)
	assert.Positive(t, usage.OutputTokens)
}
