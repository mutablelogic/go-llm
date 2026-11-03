package eliza_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	eliza "github.com/mutablelogic/go-llm/provider/eliza"
	schema "github.com/mutablelogic/go-llm/provider/schema"
	tool "github.com/mutablelogic/go-llm/toolkit/tool"
	jsonschema "github.com/mutablelogic/go-server/pkg/jsonschema"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

// user returns a message from the user with the text
func user(t *testing.T, text string) llm.Message {
	t.Helper()
	message, err := schema.NewMessage("user", text)
	require.NoError(t, err)
	return message
}

// waveTool is a tool, which eliza can't call
type waveTool struct{ tool.Base }

var _ llm.Tool = waveTool{}

func (waveTool) Name() string                                      { return "wave" }
func (waveTool) Description() string                               { return "Waves" }
func (waveTool) InputSchema() *jsonschema.Schema                   { return nil }
func (waveTool) Run(context.Context, json.RawMessage) (any, error) { return nil, nil }

func Test_Models(t *testing.T) {
	ctx := context.Background()
	client, err := eliza.New(eliza.WithSeed(42))
	require.NoError(t, err)

	t.Run("List", func(t *testing.T) {
		// A model for each language, in order of their names
		models, err := client.ListModels(ctx)
		require.NoError(t, err)
		var names []string
		for _, model := range models {
			names = append(names, model.Name())
			assert.Equal(t, client, model.Provider())
		}
		assert.Equal(t, []string{"eliza-1966-de", "eliza-1966-en", "eliza-1966-fr"}, names)
	})

	t.Run("Get", func(t *testing.T) {
		// eliza is the English model
		model, err := client.GetModel(ctx, eliza.ModelName)
		require.NoError(t, err)
		assert.Equal(t, "eliza-1966-en", model.Name())
		assert.Equal(t, []string{eliza.ModelName}, model.Aliases())

		model, err = client.GetModel(ctx, "eliza-1966-de")
		require.NoError(t, err)
		assert.Empty(t, model.Aliases())
		assert.Equal(t, `"de"`, string(model.Meta()["language"]))

		_, err = client.GetModel(ctx, "missing")
		assert.ErrorIs(t, err, llm.ErrNotFound)
	})

	t.Run("Capabilities", func(t *testing.T) {
		// eliza replies, and thinks, without tools, and does nothing else
		model, err := client.GetModel(ctx, eliza.ModelName)
		require.NoError(t, err)
		yes, no := true, false
		assert.Equal(t, llm.ModelCap{
			Completion: &yes, Tools: &no, Thinking: &yes, Vision: &no, OCR: &no,
			Transcription: &no, Translation: &no, Embedding: &no,
		}, model.Capabilities())
		assert.Equal(t, "completion, thinking", model.Capabilities().String())

		// Each of which can be changed without changing the others
		capabilities := model.Capabilities()
		*capabilities.Tools = true
		assert.Equal(t, "completion, tools, thinking", capabilities.String())
	})
}

func Test_WithoutSession(t *testing.T) {
	ctx := context.Background()
	client, err := eliza.New(eliza.WithSeed(42))
	require.NoError(t, err)
	model, err := client.GetModel(ctx, eliza.ModelName)
	require.NoError(t, err)

	t.Run("Reply", func(t *testing.T) {
		// Eliza replies to the user's message, and the tokens are estimated
		reply, usage, err := client.WithoutSession(ctx, model, []llm.Message{user(t, "I feel sad")}, nil, nil)
		require.NoError(t, err)
		assert.Equal(t, "assistant", reply.Role())
		assert.NotEmpty(t, reply.Text())
		assert.Equal(t, schema.ResultStop, reply.(*schema.Message).Result)
		assert.Equal(t, uint(3), usage.InputTokens)
		assert.NotZero(t, usage.OutputTokens)

		// And the same messages, with the same seed, have the same reply
		again, _, err := client.WithoutSession(ctx, model, []llm.Message{user(t, "I feel sad")}, nil, nil)
		require.NoError(t, err)
		assert.Equal(t, reply.Text(), again.Text())
	})

	t.Run("Stream", func(t *testing.T) {
		// The reply is streamed a word at a time, which make its text
		var pieces []string
		reply, _, err := client.WithoutSession(ctx, model, []llm.Message{user(t, "Hello")}, nil, func(kind llm.StreamType, text string) {
			assert.Equal(t, llm.StreamText, kind)
			pieces = append(pieces, text)
		})
		require.NoError(t, err)
		assert.Len(t, pieces, len(strings.Fields(reply.Text())))
		assert.Equal(t, reply.Text(), strings.Join(pieces, ""))
	})

	t.Run("Thinking", func(t *testing.T) {
		// With thinking, eliza thinks about what it remembers of the user's
		// earlier messages, which is streamed before the text
		messages := []llm.Message{
			user(t, "I feel lonely since my dog died"),
			&schema.Message{MessageMeta: schema.MessageMeta{Sender: "assistant", Content: []schema.ContentBlock{}}},
			user(t, "What should I do?"),
		}
		var kinds []llm.StreamType
		reply, _, err := client.WithoutSession(ctx, model, messages, nil, func(kind llm.StreamType, _ string) {
			kinds = append(kinds, kind)
		}, llm.WithThinking(true))
		require.NoError(t, err)
		thinking := reply.(*schema.Message).Thinking()
		assert.Contains(t, thinking, "Recalled memories from conversation")
		if assert.NotEmpty(t, kinds) {
			assert.Equal(t, llm.StreamThinking, kinds[0])
			assert.Equal(t, llm.StreamText, kinds[len(kinds)-1])
		}

		// Without it, there's no thinking
		reply, _, err = client.WithoutSession(ctx, model, messages, nil, nil)
		require.NoError(t, err)
		assert.Empty(t, reply.(*schema.Message).Thinking())
	})

	t.Run("Cancelled", func(t *testing.T) {
		// Streaming stops when the context is cancelled
		cancelled, cancel := context.WithCancel(ctx)
		cancel()
		_, _, err := client.WithoutSession(cancelled, model, []llm.Message{user(t, "Hello")}, nil, func(llm.StreamType, string) {})
		assert.ErrorIs(t, err, context.Canceled)
	})

	t.Run("Errors", func(t *testing.T) {
		other, err := eliza.New()
		require.NoError(t, err)
		otherModel, err := other.GetModel(ctx, eliza.ModelName)
		require.NoError(t, err)
		assistant := &schema.Message{MessageMeta: schema.MessageMeta{Sender: "assistant"}}

		for name, test := range map[string]struct {
			model    llm.Model
			messages []llm.Message
			tools    []llm.Tool
			want     error
		}{
			"OtherModel": {otherModel, []llm.Message{user(t, "Hello")}, nil, llm.ErrBadParameter},
			"NoMessages": {model, nil, nil, llm.ErrBadParameter},
			"NotUser":    {model, []llm.Message{assistant}, nil, llm.ErrBadParameter},
			"NoText":     {model, []llm.Message{user(t, "  ")}, nil, llm.ErrBadParameter},
			"Tools":      {model, []llm.Message{user(t, "Hello")}, []llm.Tool{waveTool{}}, llm.ErrNotImplemented},
		} {
			_, _, err := client.WithoutSession(ctx, test.model, test.messages, test.tools, nil)
			assert.ErrorIs(t, err, test.want, name)
		}
	})
}
