package gemini_test

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	anthropic "github.com/mutablelogic/go-llm/provider/anthropic"
	gemini "github.com/mutablelogic/go-llm/provider/gemini"
	schema "github.com/mutablelogic/go-llm/provider/schema"
	jsonschema "github.com/mutablelogic/go-server/pkg/jsonschema"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

// cityWeather is a tool which returns the weather in a city: Berlin is 21
// degrees, and Paris 25
type cityWeather struct{}

func (cityWeather) Name() string        { return "weather" }
func (cityWeather) Description() string { return "Returns the weather in a city" }
func (cityWeather) InputSchema() *jsonschema.Schema {
	return jsonschema.MustFor[struct {
		City string `json:"city" help:"Name of the city"`
	}]()
}
func (cityWeather) OutputSchema() *jsonschema.Schema { return nil }
func (cityWeather) Timeout() time.Duration           { return 0 }
func (cityWeather) Run(_ context.Context, input json.RawMessage) (any, error) {
	var req struct {
		City string `json:"city"`
	}
	if err := json.Unmarshal(input, &req); err != nil {
		return nil, err
	}
	celsius := 21
	if strings.Contains(strings.ToLower(req.City), "paris") {
		celsius = 25
	}
	return map[string]any{"city": req.City, "weather": "sunny", "celsius": celsius}, nil
}

// turn asks the model, with the conversation, calling the tools it asks
// for, until it replies, and returns the conversation, with its calls, their
// results, and its reply
func turn(t *testing.T, generator llm.Generator, model llm.Model, conversation []llm.Message, tools []llm.Tool) []llm.Message {
	t.Helper()
	ctx := context.Background()
	for range 4 {
		reply, _, err := generator.WithoutSession(ctx, model, conversation, tools, nil)
		require.NoError(t, err, model.Name())
		conversation = append(conversation, reply)
		calls := reply.ToolCalls()
		if len(calls) == 0 {
			return conversation
		}
		results := &schema.Message{MessageMeta: schema.MessageMeta{Sender: "tool"}}
		for _, call := range calls {
			output, err := cityWeather{}.Run(ctx, call.Input)
			require.NoError(t, err)
			data, err := json.Marshal(output)
			require.NoError(t, err)
			results.Content = append(results.Content, schema.ContentBlock{ToolResult: &schema.ToolResult{ID: call.ID, Name: call.Name, Content: data}})
		}
		conversation = append(conversation, results)
	}
	t.Fatalf("%s didn't reply", model.Name())
	return nil
}

// Test_LiveSwitch has a conversation which changes its model, from Claude
// to Gemini, and back, each calling the tool, so each is sent the other's
// calls, their results, and its thinking, with the keys in
// ANTHROPIC_API_KEY and GEMINI_API_KEY, which are paid for, or skips, when
// either isn't set, or the tests are short
func Test_LiveSwitch(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping a test of the Anthropic and Gemini APIs in short mode")
	}
	anthropicKey, geminiKey := os.Getenv("ANTHROPIC_API_KEY"), os.Getenv("GEMINI_API_KEY")
	if anthropicKey == "" || geminiKey == "" {
		t.Skip("ANTHROPIC_API_KEY and GEMINI_API_KEY not both set")
	}
	ctx := context.Background()
	claude, err := anthropic.New("", anthropicKey)
	require.NoError(t, err)
	claudeModel, err := claude.GetModel(ctx, "claude-haiku-5-5")
	require.NoError(t, err)
	google, err := gemini.New("", geminiKey)
	require.NoError(t, err)
	geminiModel, err := google.GetModel(ctx, "gemini-3.5-flash-lite")
	require.NoError(t, err)
	tools := []llm.Tool{cityWeather{}}
	ask := func(conversation []llm.Message, text string) []llm.Message {
		message, err := schema.NewMessage("user", text)
		require.NoError(t, err)
		return append(conversation, message)
	}

	// Claude calls the tool for Berlin
	conversation := ask(nil, "What's the weather in Berlin? Use the tool.")
	conversation = turn(t, claude, claudeModel, conversation, tools)
	assert.Contains(t, conversation[len(conversation)-1].Text(), "21", "Claude says Berlin's weather")

	// Gemini, which is sent Claude's call, and result, as text, calls the
	// tool for Paris, with its signature
	conversation = ask(conversation, "And in Paris? Use the tool.")
	conversation = turn(t, google, geminiModel, conversation, tools)
	assert.Contains(t, conversation[len(conversation)-1].Text(), "25", "Gemini says Paris's weather")

	// Claude, which is sent Gemini's calls, and results, says which was
	// warmer, which needs both
	conversation = ask(conversation, "Which of the two cities was warmer, and by how many degrees? Don't call the tool again.")
	conversation = turn(t, claude, claudeModel, conversation, tools)
	answer := conversation[len(conversation)-1].Text()
	assert.Contains(t, answer, "Paris")
	assert.Contains(t, answer, "4")

	// And Gemini, which is sent its own calls back, with their signatures,
	// and Claude's, as text, says it too
	conversation = ask(conversation, "Repeat the warmer city, in one word.")
	conversation = turn(t, google, geminiModel, conversation, tools)
	assert.Contains(t, conversation[len(conversation)-1].Text(), "Paris")
}
