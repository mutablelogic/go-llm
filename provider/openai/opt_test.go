package openai

import (
	"context"
	"encoding/json"
	"testing"

	// Packages
	client "github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/provider/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

func TestApply(t *testing.T) {
	t.Run("None", func(t *testing.T) {
		var r chatRequest
		require.NoError(t, r.apply(serverOther))
		assert.Equal(t, chatRequest{}, r)
	})

	t.Run("Options", func(t *testing.T) {
		var r chatRequest
		require.NoError(t, r.apply(serverOther,
			llm.WithTemperature(0.5),
			llm.WithTopP(0.9),
			llm.WithMaxTokens(100),
			llm.WithStopSequences("END"),
			llm.WithSeed(42),
			llm.WithPresencePenalty(0.1),
			llm.WithFrequencyPenalty(0.2),
			llm.WithContextWindow(8192),
			llm.WithThinkingEffort("high"),
			llm.WithUser("djt"),
			llm.WithServiceTier("auto"),
		))
		assert.Equal(t, chatRequest{
			Temperature:      types.Ptr(0.5),
			TopP:             types.Ptr(0.9),
			MaxTokens:        types.Ptr(uint(100)),
			Stop:             []string{"END"},
			Seed:             types.Ptr(uint(42)),
			PresencePenalty:  types.Ptr(0.1),
			FrequencyPenalty: types.Ptr(0.2),
			ReasoningEffort:  "high",
			User:             "djt",
			ServiceTier:      "auto",
		}, r)
	})

	t.Run("OpenAI", func(t *testing.T) {
		// OpenAI's own API has the most tokens as max_completion_tokens
		var r chatRequest
		require.NoError(t, r.apply(serverOpenAI, llm.WithMaxTokens(100)))
		assert.Nil(t, r.MaxTokens)
		assert.Equal(t, types.Ptr(uint(100)), r.MaxCompletionTokens)
	})

	t.Run("Thinking", func(t *testing.T) {
		// DeepSeek's thinking mode is turned on or off
		var r chatRequest
		require.NoError(t, r.apply(serverDeepSeek, llm.WithThinking(true)))
		assert.Equal(t, &chatThinking{Type: "enabled"}, r.Thinking)
		r = chatRequest{}
		require.NoError(t, r.apply(serverDeepSeek, llm.WithThinking(false)))
		assert.Equal(t, &chatThinking{Type: "disabled"}, r.Thinking)
		data, err := json.Marshal(r)
		require.NoError(t, err)
		assert.Contains(t, string(data), `"thinking":{"type":"disabled"}`)

		// OpenAI's reasoning models have their least effort when it's off,
		// rather than the effort, and models which don't reason have none,
		// and they're sent nothing when it's on
		for model, effort := range map[string]string{
			"gpt-5.1": "none", "gpt-5.2-mini": "none", "gpt-6-luna": "none", "gpt-6-sol": "none",
			"gpt-6-astra": "low", "gpt-6.1-sol": "low",
			"gpt-5": "minimal", "gpt-5-nano": "minimal", "gpt-5-mini-2025-08-07": "minimal",
			"o1": "low", "o3-mini": "low", "o4-mini": "low",
			"gpt-4o": "", "gpt-4.1-nano": "", "gpt-3.5-turbo": "", "omni-moderation-latest": "",
		} {
			r = chatRequest{Model: model}
			require.NoError(t, r.apply(serverOpenAI, llm.WithThinking(false), llm.WithThinkingEffort("high")))
			assert.Nil(t, r.Thinking, model)
			assert.Equal(t, effort, r.ReasoningEffort, model)
		}
		r = chatRequest{}
		require.NoError(t, r.apply(serverOpenAI, llm.WithThinking(true)))
		assert.Equal(t, chatRequest{}, r)
	})

	t.Run("ReasoningWithTools", func(t *testing.T) {
		// OpenAI's models which can turn reasoning off have it off when
		// there are tools, whatever the effort, but not on other servers,
		// or without tools, or for models which can't turn it off
		tools := []chatTool{{Type: "function", Function: chatToolFunction{Name: "wave"}}}
		for model, effort := range map[string]string{
			"gpt-6-luna": "none", "gpt-5.4": "none",
			"gpt-5": "high", "o3": "high", "gpt-6-astra": "high",
		} {
			r := chatRequest{Model: model, Tools: tools}
			require.NoError(t, r.apply(serverOpenAI, llm.WithThinkingEffort("high")))
			assert.Equal(t, effort, r.ReasoningEffort, model)
		}
		r := chatRequest{Model: "gpt-6-luna", Tools: tools}
		require.NoError(t, r.apply(serverOther, llm.WithThinkingEffort("high")))
		assert.Equal(t, "high", r.ReasoningEffort)
		r = chatRequest{Model: "gpt-6-luna"}
		require.NoError(t, r.apply(serverOpenAI, llm.WithThinkingEffort("high")))
		assert.Equal(t, "high", r.ReasoningEffort)

		// Nor when the tools are removed by the tool choice
		r = chatRequest{Model: "gpt-6-luna", Tools: tools}
		require.NoError(t, r.apply(serverOpenAI, llm.WithThinkingEffort("high"), llm.WithToolChoice("none")))
		assert.Equal(t, "high", r.ReasoningEffort)
	})

	t.Run("SystemPrompt", func(t *testing.T) {
		r := chatRequest{Messages: []chatMessage{{Role: "user", Content: "Hello"}}}
		require.NoError(t, r.apply(serverOther, llm.WithSystemPrompt("Be brief")))
		assert.Equal(t, []chatMessage{{Role: "system", Content: "Be brief"}, {Role: "user", Content: "Hello"}}, r.Messages)
	})

	t.Run("JSONSchema", func(t *testing.T) {
		var r chatRequest
		require.NoError(t, r.apply(serverOther, llm.WithJSONSchema(json.RawMessage(`{"type":"object"}`))))
		data, err := json.Marshal(r.ResponseFormat)
		require.NoError(t, err)
		assert.JSONEq(t, `{"type":"json_schema","json_schema":{"name":"response","schema":{"type":"object"}}}`, string(data))
	})

	t.Run("ToolChoice", func(t *testing.T) {
		tools := []chatTool{{Type: "function", Function: chatToolFunction{Name: "wave"}}}
		for choice, expected := range map[string]string{"auto": "auto", "any": "required"} {
			r := chatRequest{Tools: tools}
			require.NoError(t, r.apply(serverOther, llm.WithToolChoice(choice)))
			assert.Equal(t, expected, r.ToolChoice, choice)
			assert.Equal(t, tools, r.Tools)
		}

		// None removes the tools, and a choice without tools isn't sent
		r := chatRequest{Tools: tools}
		require.NoError(t, r.apply(serverOther, llm.WithToolChoice("none")))
		assert.Nil(t, r.Tools)
		assert.Empty(t, r.ToolChoice)
		r = chatRequest{}
		require.NoError(t, r.apply(serverOther, llm.WithToolChoice("any")))
		assert.Empty(t, r.ToolChoice)
	})

	t.Run("Mistral", func(t *testing.T) {
		// Mistral's API has the seed as random_seed, the most tokens as
		// max_tokens, and no user, as it rejects fields it doesn't know
		var r chatRequest
		require.NoError(t, r.apply(serverMistral, llm.WithSeed(42), llm.WithMaxTokens(100)))
		assert.Equal(t, chatRequest{RandomSeed: types.Ptr(uint(42)), MaxTokens: types.Ptr(uint(100))}, r)
		data, err := json.Marshal(r)
		require.NoError(t, err)
		assert.Contains(t, string(data), `"random_seed":42`)
		assert.NotContains(t, string(data), `"seed"`)
		assert.ErrorIs(t, r.apply(serverMistral, llm.WithUser("djt")), llm.ErrBadParameter)

		// Thinking off is no reasoning effort, and on is the most, unless
		// there's an effort, which off overrides
		for _, test := range []struct {
			opts   []llm.Opt
			effort string
		}{
			{[]llm.Opt{llm.WithThinking(false)}, "none"},
			{[]llm.Opt{llm.WithThinking(true)}, "high"},
			{[]llm.Opt{llm.WithThinking(true), llm.WithThinkingEffort("medium")}, "medium"},
			{[]llm.Opt{llm.WithThinking(false), llm.WithThinkingEffort("medium")}, "none"},
			{[]llm.Opt{llm.WithThinkingEffort("medium")}, "medium"},
		} {
			r := chatRequest{}
			require.NoError(t, r.apply(serverMistral, test.opts...))
			assert.Equal(t, test.effort, r.ReasoningEffort)
		}
	})

	t.Run("Unsupported", func(t *testing.T) {
		for _, opt := range []llm.Opt{llm.WithTopK(40), llm.WithThinking(true), llm.WithThinking(false), llm.WithThinkingBudget(1000), llm.WithKeepAlive(-1), llm.WithSafePrompt(true)} {
			var r chatRequest
			assert.ErrorIs(t, r.apply(serverOther, opt), llm.ErrBadParameter)
		}
	})
}

func TestChatMessages(t *testing.T) {
	t.Run("Image", func(t *testing.T) {
		// An image is a data URL, with the text as a part
		message := &schema.Message{Sender: "user", Content: []schema.ContentBlock{
			{Text: types.Ptr("What's this?")},
			{Resource: &schema.Resource{ResourceMeta: schema.ResourceMeta{ContentType: "image/png", Data: []byte("png")}}},
		}}
		chat, err := chatMessages(context.Background(), []llm.Message{message}, false)
		require.NoError(t, err)
		require.Len(t, chat, 1)
		assert.Equal(t, []chatPart{
			{Type: "text", Text: "What's this?"},
			{Type: "image_url", ImageURL: &chatImageURL{URL: "data:image/png;base64,cG5n"}},
		}, chat[0].Content)

		// Other resources aren't supported
		message.Content[1].Resource.ContentType = "application/pdf"
		_, err = chatMessages(context.Background(), []llm.Message{message}, false)
		assert.ErrorIs(t, err, llm.ErrBadParameter)
	})

	t.Run("Thinking", func(t *testing.T) {
		// Thinking isn't sent back
		messages := []llm.Message{
			&schema.Message{Sender: "thinking", Content: []schema.ContentBlock{{Text: types.Ptr("Hmm")}}},
			&schema.Message{Sender: "assistant", Content: []schema.ContentBlock{{Thinking: types.Ptr("Hmm")}, {Text: types.Ptr("Hello")}}},
			&schema.Message{Sender: "assistant", Content: []schema.ContentBlock{{Thinking: types.Ptr("I'll wave")}, {ToolCall: &schema.ToolCall{ID: "call_1", Name: "wave"}}}},
		}
		chat, err := chatMessages(context.Background(), messages, false)
		require.NoError(t, err)
		require.Len(t, chat, 2)
		assert.Equal(t, chatMessage{Role: "assistant", Content: "Hello"}, chat[0])
		assert.Empty(t, chat[1].ReasoningContent)

		// Except with every reply, for servers which need it
		chat, err = chatMessages(context.Background(), messages, true)
		require.NoError(t, err)
		require.Len(t, chat, 2)
		assert.Equal(t, "Hmm", chat[0].ReasoningContent)
		assert.Equal(t, "I'll wave", chat[1].ReasoningContent)
	})
}

func TestNewChatResponse(t *testing.T) {
	// The reasoning, text and tool calls are content blocks, with why the
	// model stopped
	message := newChatResponse(chatChoice{
		Message:      chatMessage{Role: "assistant", ReasoningContent: "Hmm", Content: "Hello"},
		FinishReason: "length",
	}, &chatUsage{PromptTokens: 10, CompletionTokens: 5})
	assert.Equal(t, &schema.Message{
		Sender:  "assistant",
		Content: []schema.ContentBlock{{Thinking: types.Ptr("Hmm")}, {Text: types.Ptr("Hello")}},
		Tokens:  5,
		Result:  schema.ResultMaxTokens,
	}, message)

	// The tokens of a reply are without reasoning which isn't returned
	message = newChatResponse(chatChoice{Message: chatMessage{Content: "Hello"}}, &chatUsage{CompletionTokens: 50, CompletionTokensDetails: &completionTokenDetails{ReasoningTokens: 30}})
	assert.Equal(t, uint64(20), message.Tokens)

	// Unless the reasoning is returned, as DeepSeek's is
	usage := &chatUsage{CompletionTokens: 50, CompletionTokensDetails: &completionTokenDetails{ReasoningTokens: 30}}
	choice := chatChoice{Message: chatMessage{ReasoningContent: "Hmm", Content: "Hello"}}
	assert.Equal(t, uint64(50), newChatResponse(choice, usage).Tokens)
	assert.Equal(t, llm.Usage{OutputTokens: 50}, replyUsage(choice, usage))

	// Mistral's reasoning is the thinking parts of the content
	var content any
	require.NoError(t, json.Unmarshal([]byte(`[{"type":"thinking","thinking":[{"type":"text","text":"17 times "},{"type":"text","text":"23"}]},{"type":"text","text":"391"}]`), &content))
	choice = chatChoice{Message: chatMessage{Content: content}, FinishReason: "stop"}
	message = newChatResponse(choice, usage)
	assert.Equal(t, []schema.ContentBlock{{Thinking: types.Ptr("17 times 23")}, {Text: types.Ptr("391")}}, message.Content)

	// Which is returned, so its tokens are the reply's
	assert.Equal(t, uint64(50), message.Tokens)
	assert.Equal(t, llm.Usage{OutputTokens: 50}, replyUsage(choice, usage))

	// A refusal is the text, and the reply is blocked
	message = newChatResponse(chatChoice{Message: chatMessage{Content: nil, Refusal: "I can't help with that"}, FinishReason: "stop"}, nil)
	assert.Equal(t, "I can't help with that", message.Text())
	assert.Equal(t, schema.ResultBlocked, message.Result)

	for reason, result := range map[string]schema.ResultType{"stop": schema.ResultStop, "content_filter": schema.ResultBlocked, "": schema.ResultOther} {
		assert.Equal(t, result, newChatResponse(chatChoice{FinishReason: reason}, nil).Result, reason)
	}
}

func TestChatUsage(t *testing.T) {
	// OpenAI returns the cached and reasoning tokens in the details
	var usage *chatUsage
	require.NoError(t, json.Unmarshal([]byte(`{"prompt_tokens":100,"completion_tokens":50,"prompt_tokens_details":{"cached_tokens":80},"completion_tokens_details":{"reasoning_tokens":30}}`), &usage))
	assert.Equal(t, llm.Usage{InputTokens: 100, OutputTokens: 50, CachedTokens: 80, ReasoningTokens: 30}, usage.Usage())

	// DeepSeek returns its cache hits
	usage = nil
	require.NoError(t, json.Unmarshal([]byte(`{"prompt_tokens":100,"completion_tokens":50,"prompt_cache_hit_tokens":64,"prompt_cache_miss_tokens":36}`), &usage))
	assert.Equal(t, llm.Usage{InputTokens: 100, OutputTokens: 50, CachedTokens: 64}, usage.Usage())

	// Without usage, there are no tokens
	usage = nil
	assert.Equal(t, llm.Usage{}, usage.Usage())
}

func TestEmbedApply(t *testing.T) {
	// The dimensions are OpenAI's dimensions, and Mistral's
	// output_dimension, and Mistral has no user
	var r embedRequest
	require.NoError(t, r.apply(serverOpenAI, llm.WithDimensions(256), llm.WithUser("djt")))
	assert.Equal(t, embedRequest{Dimensions: 256, User: "djt"}, r)
	r = embedRequest{}
	require.NoError(t, r.apply(serverMistral, llm.WithDimensions(256)))
	assert.Equal(t, embedRequest{OutputDimension: 256}, r)
	assert.ErrorIs(t, r.apply(serverMistral, llm.WithUser("djt")), llm.ErrBadParameter)
}

func TestChatStreamThinking(t *testing.T) {
	// Mistral streams its reasoning as thinking parts of the content, and
	// its text as strings, with the usage in the chunk which is done
	var pieces []string
	s := &chatStream{stream: func(kind llm.StreamType, piece string) {
		pieces = append(pieces, string(kind)+":"+piece)
	}}
	for _, data := range []string{
		`{"id":"1","choices":[{"index":0,"delta":{"role":"assistant","content":""}}]}`,
		`{"choices":[{"index":0,"delta":{"content":[{"type":"thinking","thinking":[{"type":"text","text":"Hmm"}]}]}}]}`,
		`{"choices":[{"index":0,"delta":{"content":"4"}}]}`,
		`{"choices":[{"index":0,"delta":{"content":""},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":5,"completion_tokens_details":{"reasoning_tokens":3}}}`,
	} {
		require.NoError(t, s.add(client.TextStreamEvent{Data: data}))
	}
	assert.Equal(t, []string{"thinking:Hmm", "text:4"}, pieces)
	response := s.Response()
	assert.Equal(t, "Hmm", response.Choices[0].Message.ReasoningContent)
	assert.Equal(t, "4", response.Choices[0].Message.Content)
	assert.Equal(t, llm.Usage{InputTokens: 10, OutputTokens: 5, ReasoningTokens: 3}, response.Usage.Usage())
	assert.True(t, s.done)
}
