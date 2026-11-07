package gemini

import (
	"context"
	"encoding/json"
	"testing"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/provider/schema"
	jsonschema "github.com/mutablelogic/go-server/pkg/jsonschema"
	types "github.com/mutablelogic/go-server/pkg/types"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

// weather is a tool which returns the weather in a city
type weather struct{}

func (weather) Name() string        { return "weather" }
func (weather) Description() string { return "The weather in a city" }
func (weather) InputSchema() *jsonschema.Schema {
	return jsonschema.MustFor[struct {
		City string `json:"city"`
	}]()
}
func (weather) OutputSchema() *jsonschema.Schema                  { return nil }
func (weather) Run(context.Context, json.RawMessage) (any, error) { return "sunny", nil }

// chatClient returns a client of the fake API, and its model which thinks
func chatClient(t *testing.T) (*fakeAPI, *Client, llm.Model) {
	t.Helper()
	api := newFakeAPI(t)
	c, err := New(api.URL, "key")
	require.NoError(t, err)
	model, err := c.GetModel(context.Background(), "gemini-new")
	require.NoError(t, err)
	return api, c, model
}

func text(t *testing.T, role, value string) llm.Message {
	t.Helper()
	message, err := schema.NewMessage(role, value)
	require.NoError(t, err)
	return message
}

func Test_Reply(t *testing.T) {
	ctx := context.Background()
	api, c, model := chatClient(t)

	// The reply is streamed, its thinking, and its text, in pieces, with the
	// tokens used
	api.reply = []string{
		`{"candidates":[{"content":{"role":"model","parts":[{"text":"Thinking about it","thought":true}]}}]}`,
		`{"candidates":[{"content":{"role":"model","parts":[{"text":"Hello, "}]}}]}`,
		`{"candidates":[{"content":{"role":"model","parts":[{"text":"David."}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":10,"cachedContentTokenCount":4,"candidatesTokenCount":3,"thoughtsTokenCount":5}}`,
	}
	var streamed []string
	reply, usage, err := c.WithoutSession(ctx, model, []llm.Message{text(t, "user", "Hi")}, nil, func(what llm.StreamType, piece string) {
		streamed = append(streamed, string(what)+":"+piece)
	})
	require.NoError(t, err)
	assert.Equal(t, []string{"thinking:Thinking about it", "text:Hello, ", "text:David."}, streamed)
	message := reply.(*schema.Message)
	assert.Equal(t, "Hello, David.", message.Text())
	require.Len(t, message.Content, 2)
	assert.Equal(t, "Thinking about it", types.Value(message.Content[0].Thinking))
	assert.Equal(t, schema.ResultStop, message.Result)
	assert.Equal(t, llm.Usage{InputTokens: 10, OutputTokens: 8, CachedTokens: 4, ReasoningTokens: 5}, usage)

	// The request has the conversation, and the system prompt, and thoughts
	// are asked for, as the model thinks
	assert.Equal(t, []content{{Role: "user", Parts: []part{{Text: "Hi"}}}}, api.asked.Contents)
	require.NotNil(t, api.asked.GenerationConfig)
	assert.True(t, api.asked.GenerationConfig.ThinkingConfig.IncludeThoughts)

	// A blocked prompt, and an error, which ends the stream
	api.reply = []string{`{"promptFeedback":{"blockReason":"SAFETY"}}`}
	reply, _, err = c.WithoutSession(ctx, model, []llm.Message{text(t, "user", "Hi")}, nil, nil)
	require.NoError(t, err)
	assert.Equal(t, schema.ResultBlocked, reply.(*schema.Message).Result)
	api.reply = []string{`{"error":{"code":429,"message":"slow down","status":"RESOURCE_EXHAUSTED"}}`}
	_, _, err = c.WithoutSession(ctx, model, []llm.Message{text(t, "user", "Hi")}, nil, nil)
	assert.ErrorIs(t, err, llm.ErrServiceUnavailable)
	api.reply = []string{`{"candidates":[{"content":{"parts":[{"text":"Cut"}]}}]}`}
	_, _, err = c.WithoutSession(ctx, model, []llm.Message{text(t, "user", "Hi")}, nil, nil)
	assert.ErrorIs(t, err, llm.ErrServiceUnavailable, "a reply which didn't finish")
}

func Test_ToolCalls(t *testing.T) {
	ctx := context.Background()
	api, c, model := chatClient(t)

	// A call of a tool, with its signature, and without an identifier, which
	// is given one, and the tools, as functions
	api.reply = []string{
		`{"candidates":[{"content":{"role":"model","parts":[{"functionCall":{"name":"weather","args":{"city":"Berlin"}},"thoughtSignature":"SIG1"}]},"finishReason":"STOP"}]}`,
	}
	reply, _, err := c.WithoutSession(ctx, model, []llm.Message{text(t, "user", "Weather?")}, []llm.Tool{weather{}}, nil, llm.WithToolChoice("auto"))
	require.NoError(t, err)
	message := reply.(*schema.Message)
	assert.Equal(t, schema.ResultToolCall, message.Result)
	require.Len(t, message.ToolCalls(), 1)
	call := message.Content[0].ToolCall
	assert.Equal(t, "weather", call.Name)
	assert.JSONEq(t, `{"city":"Berlin"}`, string(call.Input))
	assert.NotEmpty(t, call.ID)
	assert.Equal(t, map[string]any{"signature": "SIG1"}, call.Meta[metaKey])
	require.Len(t, api.asked.Tools, 1)
	assert.Equal(t, "weather", api.asked.Tools[0].FunctionDeclarations[0].Name)
	assert.Equal(t, "AUTO", api.asked.ToolConfig.FunctionCallingConfig.Mode)

	// The call is sent back, as a call, with its signature, without the
	// identifier the API didn't give, and its result, as a response, which
	// is an object
	result := &schema.Message{MessageMeta: schema.MessageMeta{Sender: "tool", Content: []schema.ContentBlock{{ToolResult: &schema.ToolResult{ID: call.ID, Name: "weather", Content: json.RawMessage(`"sunny"`)}}}}}
	api.reply = []string{`{"candidates":[{"content":{"parts":[{"text":"It's sunny."}]},"finishReason":"STOP"}]}`}
	_, _, err = c.WithoutSession(ctx, model, []llm.Message{text(t, "user", "Weather?"), message, result}, []llm.Tool{weather{}}, nil)
	require.NoError(t, err)
	require.Len(t, api.asked.Contents, 3)
	sent := api.asked.Contents[1]
	assert.Equal(t, "model", sent.Role)
	assert.Equal(t, "SIG1", sent.Parts[0].ThoughtSignature)
	assert.Equal(t, &functionCall{Name: "weather", Args: json.RawMessage(`{"city":"Berlin"}`)}, sent.Parts[0].FunctionCall)
	response := api.asked.Contents[2].Parts[0].FunctionResponse
	require.NotNil(t, response)
	assert.Equal(t, "weather", response.Name)
	assert.JSONEq(t, `{"result":"sunny"}`, string(response.Response))

	// A signature in a piece of its own is the call's before it, and a
	// result without its tool's name has its call's
	api.reply = []string{
		`{"candidates":[{"content":{"role":"model","parts":[{"functionCall":{"name":"weather","args":{"city":"Paris"}}}]}}]}`,
		`{"candidates":[{"content":{"role":"model","parts":[{"thoughtSignature":"SIG2"}]},"finishReason":"STOP"}]}`,
	}
	reply, _, err = c.WithoutSession(ctx, model, []llm.Message{text(t, "user", "Paris?")}, []llm.Tool{weather{}}, nil)
	require.NoError(t, err)
	late := reply.(*schema.Message)
	require.Len(t, late.Content, 1)
	assert.Equal(t, map[string]any{"signature": "SIG2"}, late.Content[0].ToolCall.Meta[metaKey])
	nameless := &schema.Message{MessageMeta: schema.MessageMeta{Sender: "tool", Content: []schema.ContentBlock{{ToolResult: &schema.ToolResult{ID: late.Content[0].ToolCall.ID, Content: json.RawMessage(`"warm"`)}}}}}
	api.reply = []string{`{"candidates":[{"content":{"parts":[{"text":"Warm."}]},"finishReason":"STOP"}]}`}
	_, _, err = c.WithoutSession(ctx, model, []llm.Message{text(t, "user", "Paris?"), late, nameless}, []llm.Tool{weather{}}, nil)
	require.NoError(t, err)
	assert.Equal(t, "SIG2", api.asked.Contents[1].Parts[0].ThoughtSignature)
	assert.Equal(t, "weather", api.asked.Contents[2].Parts[0].FunctionResponse.Name)
}

func Test_OtherModels(t *testing.T) {
	// A conversation which another model had: its calls, which Gemini 3
	// doesn't accept without its signatures, aren't sent, and their results
	// are text, which says what was called, with what, and what it returned,
	// and its thinking, with its signature, isn't sent
	ctx := context.Background()
	api, c, model := chatClient(t)
	claude := &schema.Message{MessageMeta: schema.MessageMeta{Sender: "assistant", Content: []schema.ContentBlock{
		{Thinking: types.Ptr("I should look"), Signature: types.Ptr("ANTHROPIC")},
		{Text: types.Ptr("Let me look.")},
		{ToolCall: &schema.ToolCall{ID: "toolu_1", Name: "weather", Input: json.RawMessage(`{"city":"Paris"}`)}},
	}}}
	failed := &schema.Message{MessageMeta: schema.MessageMeta{Sender: "tool", Content: []schema.ContentBlock{
		{ToolResult: &schema.ToolResult{ID: "toolu_1", Name: "weather", Content: json.RawMessage(`"rain"`)}},
	}}}
	onlyCall := &schema.Message{MessageMeta: schema.MessageMeta{Sender: "assistant", Content: []schema.ContentBlock{
		{ToolCall: &schema.ToolCall{ID: "toolu_2", Name: "weather", Input: json.RawMessage(`{"city":"Rome"}`)}},
	}}}
	failedToo := &schema.Message{MessageMeta: schema.MessageMeta{Sender: "tool", Content: []schema.ContentBlock{
		{ToolResult: &schema.ToolResult{ID: "toolu_2", Name: "weather", Content: json.RawMessage(`"no such city"`), IsError: true}},
	}}}
	api.reply = []string{`{"candidates":[{"content":{"parts":[{"text":"Rain, then."}]},"finishReason":"STOP"}]}`}
	_, _, err := c.WithoutSession(ctx, model, []llm.Message{
		text(t, "system", "Be brief."), text(t, "user", "Weather in Paris?"), claude, failed, onlyCall, failedToo,
	}, []llm.Tool{weather{}}, nil, llm.WithSystemPrompt("You are Omodie."))
	require.NoError(t, err)
	assert.Equal(t, "You are Omodie.\n\nBe brief.", api.asked.SystemInstruction.Parts[0].Text)
	assert.Equal(t, []content{
		{Role: "user", Parts: []part{{Text: "Weather in Paris?"}}},
		{Role: "model", Parts: []part{{Text: "Let me look."}}},
		{Role: "user", Parts: []part{
			{Text: `(The tool weather was called with {"city":"Paris"}, and returned: rain)`},
			{Text: `(The tool weather was called with {"city":"Rome"}, and failed: no such city)`},
		}},
	}, api.asked.Contents)
}

func Test_Thinking(t *testing.T) {
	// Gemini 2.5 thinks within a budget, or not, apart from Pro, which can't
	// stop, and later models at a level, or the lowest, and a model which
	// doesn't think can't be asked to
	think := func(name string, thinks bool, asked, on bool, effort string) (*thinkingConfig, error) {
		m := &Model{data: apiModel{Name: "models/" + name, Thinking: types.Ptr(thinks)}}
		return m.thinking(asked, on, effort)
	}
	config, err := think("gemini-2.5-flash", true, true, false, "")
	require.NoError(t, err)
	assert.Equal(t, &thinkingConfig{ThinkingBudget: types.Ptr(0)}, config)
	config, err = think("gemini-2.5-pro", true, true, false, "")
	require.NoError(t, err)
	assert.Equal(t, &thinkingConfig{ThinkingBudget: types.Ptr(128)}, config)
	config, err = think("gemini-2.5-flash", true, false, false, "medium")
	require.NoError(t, err)
	assert.Equal(t, &thinkingConfig{IncludeThoughts: true, ThinkingBudget: types.Ptr(8192)}, config)
	config, err = think("gemini-3.5-flash", true, false, false, "high")
	require.NoError(t, err)
	assert.Equal(t, &thinkingConfig{IncludeThoughts: true, ThinkingLevel: "HIGH"}, config)
	config, err = think("gemini-3.5-flash", true, true, false, "")
	require.NoError(t, err)
	assert.Equal(t, &thinkingConfig{ThinkingLevel: "LOW"}, config)
	config, err = think("gemini-3.5-flash", true, false, false, "")
	require.NoError(t, err)
	assert.Equal(t, &thinkingConfig{IncludeThoughts: true}, config)
	config, err = think("gemini-old", false, true, false, "")
	require.NoError(t, err)
	assert.Nil(t, config)
	_, err = think("gemini-old", false, true, true, "")
	assert.ErrorIs(t, err, llm.ErrBadParameter)
	_, err = think("gemini-old", false, false, false, "high")
	assert.ErrorIs(t, err, llm.ErrBadParameter, "an effort asks it to think")
	config, err = think("gemini-old", false, true, false, "high")
	require.NoError(t, err)
	assert.Nil(t, config, "unless thinking is off")
}

func Test_Options(t *testing.T) {
	ctx := context.Background()
	api, c, model := chatClient(t)
	api.reply = []string{`{"candidates":[{"content":{"parts":[{"text":"{}"}]},"finishReason":"STOP"}]}`}

	// The sampling options, the most tokens, stop sequences, a seed, a
	// schema, and options the API hasn't, which are ignored
	_, _, err := c.WithoutSession(ctx, model, []llm.Message{text(t, "user", "Hi")}, nil, nil,
		llm.WithTemperature(0.5), llm.WithTopP(0.9), llm.WithTopK(40), llm.WithMaxTokens(100),
		llm.WithStopSequences("END"), llm.WithOpt(llm.SeedKey, uint(7)),
		llm.WithJSONSchema(json.RawMessage(`{"type":"object"}`)),
		llm.WithOpt(llm.UserKey, "david"), llm.WithOpt(llm.ServiceTierKey, "auto"))
	require.NoError(t, err)
	config := api.asked.GenerationConfig
	require.NotNil(t, config)
	assert.Equal(t, 0.5, types.Value(config.Temperature))
	assert.Equal(t, 0.9, types.Value(config.TopP))
	assert.Equal(t, uint(40), types.Value(config.TopK))
	assert.Equal(t, uint(100), config.MaxOutputTokens)
	assert.Equal(t, []string{"END"}, config.StopSequences)
	assert.Equal(t, uint(7), types.Value(config.Seed))
	assert.Equal(t, "application/json", config.ResponseMimeType)
	assert.JSONEq(t, `{"type":"object"}`, string(config.ResponseJSONSchema))
	assert.Nil(t, api.asked.ToolConfig, "a tool choice needs tools")

	// An option which isn't for replies, a tool choice which isn't one, and
	// an effort which isn't one
	for _, opt := range []llm.Opt{llm.WithOpt(llm.TaskTypeKey, "x"), llm.WithToolChoice("sometimes"), llm.WithOpt(llm.ThinkingEffortKey, "huge"), llm.WithOpt(llm.JSONSchemaKey, json.RawMessage("{"))} {
		_, _, err = c.WithoutSession(ctx, model, []llm.Message{text(t, "user", "Hi")}, nil, nil, opt)
		assert.ErrorIs(t, err, llm.ErrBadParameter)
	}
}

func Test_Accepts(t *testing.T) {
	c := &Client{}
	for _, contentType := range []string{"image/png", "audio/mpeg", "video/mp4", "application/pdf", "text/plain; charset=utf-8"} {
		assert.True(t, c.Accepts(nil, contentType), contentType)
	}
	assert.False(t, c.Accepts(nil, "application/zip"))
}

func Test_Replay(t *testing.T) {
	// A reply's calls Gemini made are sent back as calls, unless the model
	// needs their signatures, and none has one, when they're text, and calls
	// made together, of which only the first is signed, are sent back
	ctx := context.Background()
	call := func(id, city, signature string) schema.ContentBlock {
		meta := callMeta{Signature: signature}
		return schema.ContentBlock{ToolCall: &schema.ToolCall{ID: id, Name: "weather", Input: json.RawMessage(`{"city":"` + city + `"}`), Meta: map[string]any{metaKey: meta.value()}}}
	}
	result := func(id, value string) schema.ContentBlock {
		return schema.ContentBlock{ToolResult: &schema.ToolResult{ID: id, Name: "weather", Content: json.RawMessage(`"` + value + `"`)}}
	}
	conversation := []llm.Message{
		text(t, "user", "Weather?"),
		&schema.Message{MessageMeta: schema.MessageMeta{Sender: "assistant", Content: []schema.ContentBlock{call("a", "Oslo", "")}}},
		&schema.Message{MessageMeta: schema.MessageMeta{Sender: "tool", Content: []schema.ContentBlock{result("a", "snow")}}},
		&schema.Message{MessageMeta: schema.MessageMeta{Sender: "assistant", Content: []schema.ContentBlock{call("b", "Rome", "SIG"), call("c", "Bonn", "")}}},
		&schema.Message{MessageMeta: schema.MessageMeta{Sender: "tool", Content: []schema.ContentBlock{result("b", "sun"), result("c", "rain")}}},
	}
	model := func(name string) *Model { return &Model{data: apiModel{Name: "models/" + name}} }

	// Gemini 3 needs signatures: the unsigned reply is text, and the signed
	// one is calls, with its unsigned sibling
	contents, _, err := newContents(ctx, model("gemini-3.5-flash"), conversation)
	require.NoError(t, err)
	require.Len(t, contents, 3)
	assert.Equal(t, "user", contents[0].Role)
	assert.Equal(t, `(The tool weather was called with {"city":"Oslo"}, and returned: snow)`, contents[0].Parts[1].Text)
	assert.Equal(t, "model", contents[1].Role)
	require.Len(t, contents[1].Parts, 2)
	assert.Equal(t, "SIG", contents[1].Parts[0].ThoughtSignature)
	assert.JSONEq(t, `{"city":"Bonn"}`, string(contents[1].Parts[1].FunctionCall.Args), "the unsigned sibling")
	require.Len(t, contents[2].Parts, 2)
	assert.NotNil(t, contents[2].Parts[0].FunctionResponse)
	assert.NotNil(t, contents[2].Parts[1].FunctionResponse)

	// Gemini 2.5 doesn't: every call is sent back
	contents, _, err = newContents(ctx, model("gemini-2.5-flash"), conversation)
	require.NoError(t, err)
	require.Len(t, contents, 5)
	assert.NotNil(t, contents[1].Parts[0].FunctionCall)
	assert.NotNil(t, contents[2].Parts[0].FunctionResponse)
}
