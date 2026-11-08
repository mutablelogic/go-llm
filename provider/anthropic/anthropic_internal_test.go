package anthropic

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	// Packages
	client "github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/provider/schema"
	jsonschema "github.com/mutablelogic/go-server/pkg/jsonschema"
	types "github.com/mutablelogic/go-server/pkg/types"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

///////////////////////////////////////////////////////////////////////////////
// HELPERS

// adaptiveCaps are the capabilities of a model which thinks adaptively, and
// has effort, and enabledCaps those of an older model, which thinks within
// a budget
const (
	adaptiveCaps = `{"image_input":{"supported":true},"structured_outputs":{"supported":true},"thinking":{"supported":true,"types":{"adaptive":{"supported":true},"enabled":{"supported":false}}},"effort":{"supported":true}}`
	enabledCaps  = `{"image_input":{"supported":true},"thinking":{"supported":true,"types":{"adaptive":{"supported":false},"enabled":{"supported":true}}},"effort":{"supported":false}}`
)

// fakeAPI is a fake of Anthropic's API, which lists two models, a page at a
// time, streams a reply, which thinks, replies, and calls a tool, and counts
// tokens, as a token for each byte of the text, and two for the message
type fakeAPI struct {
	*httptest.Server
	request  messagesRequest // The last request to POST /v1/messages
	header   http.Header     // Its headers
	counts   atomic.Int32    // How many times tokens were counted
	failures atomic.Int32    // How many counts fail, with 429, before they succeed
}

func newFakeAPI(t *testing.T) *fakeAPI {
	t.Helper()
	api := &fakeAPI{}
	api.Server = httptest.NewServer(http.HandlerFunc(api.serve))
	t.Cleanup(api.Close)
	return api
}

func (api *fakeAPI) serve(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("x-api-key") != "key" || r.Header.Get("anthropic-version") != APIVersion {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = io.WriteString(w, `{"type":"error","error":{"type":"authentication_error","message":"invalid x-api-key"}}`)
		return
	}
	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/v1/models":
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("after_id") == "" {
			fmt.Fprintf(w, `{"data":[{"id":"claude-new","display_name":"Claude New","created_at":"2026-10-01T00:00:00Z","max_input_tokens":1000000,"max_tokens":128000,"capabilities":%s}],"has_more":true,"last_id":"claude-new"}`, adaptiveCaps)
		} else {
			fmt.Fprintf(w, `{"data":[{"id":"claude-old","display_name":"Claude Old","max_input_tokens":200000,"max_tokens":64000,"capabilities":%s}],"has_more":false,"last_id":"claude-old"}`, enabledCaps)
		}
	case r.Method == http.MethodGet && r.URL.Path == "/v1/models/claude-new":
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"id":"claude-new","display_name":"Claude New","max_input_tokens":1000000,"max_tokens":128000,"capabilities":%s}`, adaptiveCaps)
	case r.Method == http.MethodPost && r.URL.Path == "/v1/messages/count_tokens":
		api.counts.Add(1)
		w.Header().Set("Content-Type", "application/json")
		if api.failures.Load() > 0 {
			api.failures.Add(-1)
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = io.WriteString(w, `{"type":"error","error":{"type":"rate_limit_error","message":"rate limited"}}`)
			return
		}
		var request messagesRequest
		_ = json.NewDecoder(r.Body).Decode(&request)
		fmt.Fprintf(w, `{"input_tokens":%d}`, len(request.Messages[0].Content[0].Text)+2)
	case r.Method == http.MethodPost && r.URL.Path == "/v1/messages":
		api.header = r.Header.Clone()
		_ = json.NewDecoder(r.Body).Decode(&api.request)
		w.Header().Set("Content-Type", "text/event-stream")
		for _, event := range []string{
			`{"type":"message_start","message":{"id":"msg_1","model":"claude-new","role":"assistant","content":[],"usage":{"input_tokens":10,"output_tokens":1,"cache_read_input_tokens":100,"cache_creation_input_tokens":5}}}`,
			`{"type":"content_block_start","index":0,"content_block":{"type":"thinking","thinking":"","signature":""}}`,
			`{"type":"content_block_delta","index":0,"delta":{"type":"thinking_delta","thinking":"Let me "}}`,
			`{"type":"content_block_delta","index":0,"delta":{"type":"thinking_delta","thinking":"check."}}`,
			`{"type":"content_block_delta","index":0,"delta":{"type":"signature_delta","signature":"sig"}}`,
			`{"type":"content_block_stop","index":0}`,
			`{"type":"content_block_start","index":1,"content_block":{"type":"text","text":""}}`,
			`{"type":"content_block_delta","index":1,"delta":{"type":"text_delta","text":"It's "}}`,
			`{"type":"content_block_delta","index":1,"delta":{"type":"text_delta","text":"sunny."}}`,
			`{"type":"content_block_stop","index":1}`,
			`{"type":"content_block_start","index":2,"content_block":{"type":"tool_use","id":"toolu_1","name":"weather","input":{}}}`,
			`{"type":"content_block_delta","index":2,"delta":{"type":"input_json_delta","partial_json":"{\"city\":"}}`,
			`{"type":"content_block_delta","index":2,"delta":{"type":"input_json_delta","partial_json":"\"Berlin\"}"}}`,
			`{"type":"content_block_stop","index":2}`,
			`{"type":"message_delta","delta":{"stop_reason":"tool_use"},"usage":{"output_tokens":42}}`,
			`{"type":"message_stop"}`,
		} {
			var e struct{ Type string }
			_ = json.Unmarshal([]byte(event), &e)
			fmt.Fprintf(w, "event: %s\ndata: %s\n\n", e.Type, event)
		}
	default:
		http.NotFound(w, r)
	}
}

// newTestClient returns a client of the fake API, and its models
func newTestClient(t *testing.T, api *fakeAPI) (*Client, *Model, *Model) {
	t.Helper()
	c, err := New(api.URL, "key")
	require.NoError(t, err)
	models, err := c.ListModels(context.Background())
	require.NoError(t, err)
	require.Len(t, models, 2)
	return c, models[0].(*Model), models[1].(*Model)
}

// weatherTool is a tool which the model can call
type weatherTool struct{}

func (weatherTool) Name() string                                      { return "weather" }
func (weatherTool) Title() string                                     { return "" }
func (weatherTool) Description() string                               { return "The weather in a city" }
func (weatherTool) InputSchema() *jsonschema.Schema                   { return nil }
func (weatherTool) OutputSchema() *jsonschema.Schema                  { return nil }
func (weatherTool) Timeout() time.Duration                            { return 0 }
func (weatherTool) Run(context.Context, json.RawMessage) (any, error) { return nil, nil }

///////////////////////////////////////////////////////////////////////////////
// TESTS

func Test_New(t *testing.T) {
	_, err := New("", "")
	assert.ErrorIs(t, err, llm.ErrBadParameter, "it needs an API key")

	c, err := New("", "key")
	require.NoError(t, err)
	assert.Equal(t, Endpoint, c.URL().String())
	assert.Equal(t, llm.Anthropic, c.Name())

	c, err = New("example.com", "key")
	require.NoError(t, err)
	assert.Equal(t, "https://example.com/v1", c.URL().String())
}

func Test_Models(t *testing.T) {
	ctx := context.Background()
	api := newFakeAPI(t)
	c, newer, older := newTestClient(t, api)

	// Every page of the models is listed, with their limits and capabilities
	assert.Equal(t, "claude-new", newer.Name())
	assert.Equal(t, "Claude New", newer.Description())
	assert.Equal(t, uint(1000000), types.Value(newer.InputTokenLimit()))
	assert.Equal(t, uint(128000), types.Value(newer.OutputTokenLimit()))
	assert.Equal(t, "chat, tools, thinking, image_input", newer.Capabilities().String())
	assert.Equal(t, "adaptive", newer.thinks())
	assert.Equal(t, "claude-old", older.Name())
	assert.Equal(t, "enabled", older.thinks())
	assert.False(t, newer.Created().IsZero())

	// A model is got by its name, and the API is pinged
	model, err := c.GetModel(ctx, "claude-new")
	require.NoError(t, err)
	assert.Equal(t, "Claude New", model.Description())
	assert.NoError(t, c.Ping(ctx))

	// A key which isn't accepted fails
	bad, err := New(api.URL, "wrong")
	require.NoError(t, err)
	assert.Error(t, bad.Ping(ctx))
}

func Test_WithoutSession(t *testing.T) {
	ctx := context.Background()
	api := newFakeAPI(t)
	c, model, _ := newTestClient(t, api)

	// The reply is streamed, with its thinking and text, and has its
	// thinking, with its signature, its text, and the tool it called
	var streamed []string
	messages := []llm.Message{
		&schema.Message{MessageMeta: schema.MessageMeta{Sender: "system", Content: []schema.ContentBlock{{Text: types.Ptr("Be brief")}}}},
		&schema.Message{MessageMeta: schema.MessageMeta{Sender: "user", Content: []schema.ContentBlock{{Text: types.Ptr("What's the weather in Berlin?")}}}},
	}
	reply, usage, err := c.WithoutSession(ctx, model, messages, []llm.Tool{weatherTool{}}, func(kind llm.StreamType, text string) {
		streamed = append(streamed, string(kind)+":"+text)
	})
	require.NoError(t, err)
	assert.Equal(t, []string{"thinking:Let me ", "thinking:check.", "text:It's ", "text:sunny."}, streamed)
	message := reply.(*schema.Message)
	require.Len(t, message.Content, 3)
	assert.Equal(t, "Let me check.", types.Value(message.Content[0].Thinking))
	assert.Equal(t, "sig", types.Value(message.Content[0].Signature))
	assert.Equal(t, "It's sunny.", reply.Text())
	require.Len(t, reply.ToolCalls(), 1)
	assert.Equal(t, "toolu_1", reply.ToolCalls()[0].ID)
	assert.JSONEq(t, `{"city":"Berlin"}`, string(reply.ToolCalls()[0].Input))
	assert.Equal(t, schema.ResultToolCall, message.Result)
	assert.Equal(t, uint64(42), message.Tokens)

	// The input tokens include those read from, and written to, the cache
	assert.Equal(t, llm.Usage{InputTokens: 115, OutputTokens: 42, CachedTokens: 100}, usage)

	// The request is streamed, and cached, with the system prompt, the
	// tools, and adaptive thinking, which is summarized, and dropped when
	// the conversation changed, with its beta
	request := api.request
	assert.True(t, request.Stream)
	assert.Equal(t, "ephemeral", request.CacheControl.Type)
	assert.Equal(t, "Be brief", request.System)
	require.Len(t, request.Tools, 1)
	assert.JSONEq(t, `{"type":"object","properties":{}}`, string(request.Tools[0].InputSchema))
	require.NotNil(t, request.Thinking)
	assert.Equal(t, "adaptive", request.Thinking.Type)
	assert.Equal(t, "summarized", request.Thinking.Display)
	assert.Equal(t, "drop_block", request.Thinking.BlockBinding.PrefixMismatchBehavior)
	assert.Equal(t, bindingBeta, api.header.Get("anthropic-beta"))
	assert.Equal(t, uint(maxTokens), request.MaxTokens)
	require.Len(t, request.Messages, 1)
	assert.Equal(t, "What's the weather in Berlin?", request.Messages[0].Content[0].Text)

	// The reply, and the tool's result, are sent back, with the reply's
	// thinking, as the model is calling tools in this turn
	messages = append(messages, reply, &schema.Message{MessageMeta: schema.MessageMeta{Sender: "tool", Content: []schema.ContentBlock{{ToolResult: &schema.ToolResult{ID: "toolu_1", Name: "weather", Content: json.RawMessage(`{"sunny":true}`)}}}}})
	_, _, err = c.WithoutSession(ctx, model, messages, []llm.Tool{weatherTool{}}, nil)
	require.NoError(t, err)
	require.Len(t, api.request.Messages, 3)
	assistant := api.request.Messages[1]
	assert.Equal(t, "assistant", assistant.Role)
	assert.Equal(t, []string{"thinking", "text", "tool_use"}, blockTypes(assistant))
	assert.Equal(t, "sig", assistant.Content[0].Signature)
	assert.Equal(t, "tool_result", api.request.Messages[2].Content[0].Type)
	assert.Equal(t, `{"sunny":true}`, api.request.Messages[2].Content[0].Content)

	// A model which isn't one of the client's isn't accepted
	_, _, err = c.WithoutSession(ctx, nil, messages, nil, nil)
	assert.ErrorIs(t, err, llm.ErrBadParameter)
}

func Test_Messages(t *testing.T) {
	ctx := context.Background()
	text := func(role, text string) llm.Message {
		return &schema.Message{MessageMeta: schema.MessageMeta{Sender: role, Content: []schema.ContentBlock{{Text: types.Ptr(text)}}}}
	}
	signed := &schema.Message{MessageMeta: schema.MessageMeta{Sender: "assistant", Content: []schema.ContentBlock{
		{Thinking: types.Ptr("Hmm"), Signature: types.Ptr("sig")},
		{Text: types.Ptr("Hello")},
	}}}

	// The system messages are the system prompt, thinking messages are left
	// out, and messages with the same role are one, so roles take turns
	messages, system, err := newMessages(ctx, []llm.Message{
		text("system", "Be brief"),
		text("user", "Hi"),
		text("thinking", "..."),
		text("user", "Are you there?"),
		signed,
		text("system", "Be kind"),
		text("user", "Bye"),
	})
	require.NoError(t, err)
	assert.Equal(t, "Be brief\n\nBe kind", system)
	require.Len(t, messages, 3)
	assert.Equal(t, []string{"text", "text"}, blockTypes(messages[0]))

	// A reply's thinking from an earlier turn isn't sent back, and a reply
	// with only thinking, which isn't sent, says there's no reply
	assert.Equal(t, []string{"text"}, blockTypes(messages[1]))
	messages, _, err = newMessages(ctx, []llm.Message{
		text("user", "Hi"),
		&schema.Message{MessageMeta: schema.MessageMeta{Sender: "assistant", Content: []schema.ContentBlock{{Thinking: types.Ptr("Hmm")}}}},
	})
	require.NoError(t, err)
	assert.Equal(t, emptyReply, messages[1].Content[0].Text, "unsigned thinking isn't sent back")

	// A conversation needs messages, other than system messages
	_, _, err = newMessages(ctx, []llm.Message{text("system", "Be brief")})
	assert.ErrorIs(t, err, llm.ErrBadParameter)
	_, _, err = newMessages(ctx, []llm.Message{text("other", "Hi")})
	assert.ErrorIs(t, err, llm.ErrBadParameter)
}

func Test_Options(t *testing.T) {
	api := newFakeAPI(t)
	_, newer, older := newTestClient(t, api)
	request := func(t *testing.T, model *Model, opts ...llm.Opt) *messagesRequest {
		t.Helper()
		r := &messagesRequest{Model: model.Name(), Tools: []tool{{Name: "weather"}}}
		require.NoError(t, r.apply(model, opts...))
		return r
	}

	// Thinking off makes the least effort, for a model which thinks
	// adaptively, rather than turning it off, which it may not accept,
	// unless there's an effort, and its thinking isn't shown, but it's still
	// dropped, when the conversation changed, as the model still thinks
	r := request(t, newer, llm.WithOpt(llm.ThinkingKey, false))
	assert.Equal(t, "adaptive", r.Thinking.Type)
	assert.Equal(t, "omitted", r.Thinking.Display)
	assert.Equal(t, "drop_block", r.Thinking.BlockBinding.PrefixMismatchBehavior)
	assert.Equal(t, "low", r.OutputConfig.Effort)
	assert.Equal(t, bindingBeta, r.betas())
	r = request(t, newer, llm.WithOpt(llm.ThinkingKey, false), llm.WithOpt(llm.ThinkingEffortKey, "high"))
	assert.Equal(t, "high", r.OutputConfig.Effort)

	// An effort is the nearest which the model has, when it says which it
	// has, and otherwise the effort
	r = request(t, newer, llm.WithOpt(llm.ThinkingEffortKey, "max"))
	assert.Equal(t, "max", r.OutputConfig.Effort)
	// as the Models API lists them, such as Claude Opus 4.6, which has max,
	// but not xhigh, and Claude Opus 4.5, which has neither
	withLevels := func(t *testing.T, levels string) *Model {
		t.Helper()
		var caps modelCapabilities
		require.NoError(t, json.Unmarshal([]byte(`{"thinking":{"supported":true,"types":{"adaptive":{"supported":true}}},"effort":`+levels+`}`), &caps))
		return &Model{caps: caps}
	}
	opus46 := withLevels(t, `{"supported":true,"low":{"supported":true},"medium":{"supported":true},"high":{"supported":true},"xhigh":{"supported":false},"max":{"supported":true}}`)
	opus45 := withLevels(t, `{"supported":true,"low":{"supported":true},"medium":{"supported":true},"high":{"supported":true},"xhigh":{"supported":false},"max":{"supported":false}}`)
	for _, test := range []struct {
		model     *Model
		ask, want string
	}{
		{opus46, "max", "max"}, {opus46, "xhigh", "max"}, {opus46, "high", "high"}, {opus46, "medium", "medium"},
		{opus45, "max", "high"}, {opus45, "xhigh", "high"}, {opus45, "low", "low"},
	} {
		r = &messagesRequest{}
		require.NoError(t, r.apply(test.model, llm.WithOpt(llm.ThinkingEffortKey, test.ask)))
		assert.Equal(t, test.want, r.OutputConfig.Effort, test.ask)
	}

	// An older model thinks only when it's asked to, within a budget, and has
	// no effort
	assert.Nil(t, request(t, older).Thinking)
	r = request(t, older, llm.WithOpt(llm.ThinkingKey, true), llm.WithOpt(llm.ThinkingEffortKey, "high"))
	assert.Equal(t, "enabled", r.Thinking.Type)
	assert.Equal(t, uint(thinkingBudget), r.Thinking.BudgetTokens)
	assert.Nil(t, r.OutputConfig)
	assert.Equal(t, uint(maxTokens), r.MaxTokens)

	// Its budget is at least the least the API accepts, and less than the
	// most tokens of its reply, which must be more than that
	r = request(t, older, llm.WithOpt(llm.ThinkingKey, true), llm.WithOpt(llm.MaxTokensKey, uint(2000)))
	assert.Equal(t, uint(minThinkingBudget), r.Thinking.BudgetTokens)
	err := (&messagesRequest{}).apply(older, llm.WithOpt(llm.ThinkingKey, true), llm.WithOpt(llm.MaxTokensKey, uint(1000)))
	assert.ErrorIs(t, err, llm.ErrBadParameter)

	// The reply's JSON schema is strict, a tool choice of any is auto, and
	// the system prompt, stop sequences, user and most tokens are set
	r = request(t, newer,
		llm.WithOpt(llm.JSONSchemaKey, json.RawMessage(`{"type":"object","properties":{"city":{"type":"object","properties":{}}}}`)),
		llm.WithOpt(llm.ToolChoiceKey, "any"),
		llm.WithOpt(llm.SystemPromptKey, "Be brief"),
		llm.WithOpt(llm.StopSequencesKey, []string{"END"}),
		llm.WithOpt(llm.UserKey, "david"),
		llm.WithOpt(llm.MaxTokensKey, uint(100)),
	)
	assert.JSONEq(t, `{"type":"object","additionalProperties":false,"properties":{"city":{"type":"object","additionalProperties":false,"properties":{}}}}`, string(r.OutputConfig.Format.Schema))
	assert.Equal(t, "auto", r.ToolChoice.Type)
	assert.Equal(t, "Be brief", r.System)
	assert.Equal(t, []string{"END"}, r.StopSequences)
	assert.Equal(t, "david", r.Metadata.UserID)
	assert.Equal(t, uint(100), r.MaxTokens)

	// Sampling options, which the models don't take, and others which
	// aren't supported, fail
	assert.False(t, types.Value(newer.Capabilities().Temperature))
	err = (&messagesRequest{}).apply(newer, llm.WithOpt(llm.TemperatureKey, 0.5))
	assert.ErrorIs(t, err, llm.ErrBadParameter)
	err = (&messagesRequest{}).apply(newer, llm.WithOpt(llm.SeedKey, uint(1)))
	assert.ErrorIs(t, err, llm.ErrBadParameter)
	err = (&messagesRequest{}).apply(newer, llm.WithOpt(llm.ToolChoiceKey, "required"))
	assert.ErrorIs(t, err, llm.ErrBadParameter)
}

func Test_Measure(t *testing.T) {
	ctx := context.Background()
	api := newFakeAPI(t)
	c, model, _ := newTestClient(t, api)

	// The tokens of the text are those of the message, less those of a
	// message with one token, apart from that token, which are counted once,
	// and text which was counted isn't counted again
	tokens, err := c.Measure(ctx, model, "hello")
	require.NoError(t, err)
	assert.Equal(t, uint(5), tokens)
	assert.Equal(t, int32(2), api.counts.Load())
	tokens, err = c.Measure(ctx, model, "hello")
	require.NoError(t, err)
	assert.Equal(t, uint(5), tokens)
	assert.Equal(t, int32(2), api.counts.Load(), "it isn't counted again")
	tokens, err = c.Measure(ctx, model, "")
	require.NoError(t, err)
	assert.Zero(t, tokens)

	// When tokens can't be counted, such as when counting is rate limited,
	// they're an estimate
	api.failures.Store(1)
	tokens, err = c.Measure(ctx, model, "twelve bytes")
	require.NoError(t, err)
	assert.Equal(t, uint(3), tokens)

	// Unless the context is done
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	_, err = c.Measure(cancelled, model, "something else")
	assert.Error(t, err)
}

func Test_StreamError(t *testing.T) {
	s := &replyStream{}
	err := s.add(clientEvent(`{"type":"error","error":{"type":"overloaded_error","message":"Overloaded"}}`))
	assert.ErrorIs(t, err, llm.ErrServiceUnavailable)
	assert.False(t, s.done)
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE

// blockTypes returns the types of the message's content blocks
func blockTypes(m message) []string {
	result := make([]string, len(m.Content))
	for i, block := range m.Content {
		result[i] = block.Type
	}
	return result
}

// clientEvent returns an event of a stream, with the data
func clientEvent(data string) client.TextStreamEvent {
	return client.TextStreamEvent{Data: strings.TrimSpace(data)}
}

func Test_StrictSchema(t *testing.T) {
	// Every object schema, within the keywords whose values are schemas, has
	// additionalProperties false, unless it says otherwise, and values which
	// aren't schemas, such as an enum's, are left as they are
	schema, err := strictSchema(json.RawMessage(`{
		"type":"object",
		"properties":{
			"place":{"anyOf":[{"type":"object","properties":{}},{"type":"string"}]},
			"tags":{"type":"array","items":{"type":"object"}},
			"shape":{"enum":[{"type":"object"}],"const":{"type":"object"},"default":{"type":"object"}},
			"open":{"type":"object","additionalProperties":true}
		},
		"$defs":{"point":{"type":"object"}}
	}`))
	require.NoError(t, err)
	assert.JSONEq(t, `{
		"type":"object","additionalProperties":false,
		"properties":{
			"place":{"anyOf":[{"type":"object","properties":{},"additionalProperties":false},{"type":"string"}]},
			"tags":{"type":"array","items":{"type":"object","additionalProperties":false}},
			"shape":{"enum":[{"type":"object"}],"const":{"type":"object"},"default":{"type":"object"}},
			"open":{"type":"object","additionalProperties":true}
		},
		"$defs":{"point":{"type":"object","additionalProperties":false}}
	}`, string(schema))

	_, err = strictSchema(json.RawMessage(`[]`))
	assert.ErrorIs(t, err, llm.ErrBadParameter)
}

func Test_ReplyBlocks(t *testing.T) {
	// A reply's blocks are sent back in the order they were generated, with
	// its thinking, and its thinking which is encrypted, from the reply
	// as it was returned
	reply := newReply(messagesResponse{StopReason: "tool_use", Content: []contentBlock{
		{Type: "thinking", Thinking: types.Ptr("First"), Signature: "sig1"},
		{Type: "text", Text: "Let me look."},
		{Type: "redacted_thinking", Data: "secret"},
		{Type: "tool_use", ID: "toolu_1", Name: "weather", Input: json.RawMessage(`{"city":"Berlin"}`)},
		{Type: "thinking", Thinking: types.Ptr("Second"), Signature: "sig2"},
		{Type: "text", Text: "And again."},
	}})
	assert.Equal(t, "secret", types.Value(reply.Content[2].Redacted))
	order := []string{"thinking", "text", "redacted_thinking", "tool_use", "thinking", "text"}

	// Whichever form of the schema's message it is
	for _, m := range []llm.Message{reply, *reply, &reply.MessageMeta, reply.MessageMeta} {
		blocks := assistantBlocks(m, true)
		assert.Equal(t, order, blockTypes(message{Content: blocks}))
		assert.Equal(t, "sig2", blocks[4].Signature)
		assert.Equal(t, "secret", blocks[2].Data)
	}

	// And without its thinking, when it isn't sent back
	assert.Equal(t, []string{"text", "tool_use", "text"}, blockTypes(message{Content: assistantBlocks(reply, false)}))
}
