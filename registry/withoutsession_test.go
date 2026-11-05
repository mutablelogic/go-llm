package registry_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	providerschema "github.com/mutablelogic/go-llm/provider/schema"
	registry "github.com/mutablelogic/go-llm/registry"
	schema "github.com/mutablelogic/go-llm/registry/schema"
	toolkit "github.com/mutablelogic/go-llm/toolkit"
	jsonschema "github.com/mutablelogic/go-server/pkg/jsonschema"
	types "github.com/mutablelogic/go-server/pkg/types"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

///////////////////////////////////////////////////////////////////////////////
// FAKES

// scripted is a provider which generates its replies in turn, by value
// when byValue is set, and records what it was asked with
type scripted struct {
	*provider
	replies []*providerschema.Message
	byValue bool
	err     error
	asked   []asked
	usage   *llm.Usage    // The usage of each reply, which is 10 input and 5 output tokens when nil
	input   *uint         // The input token limit of its models, or nil when it isn't known
	delay   time.Duration // How long it waits before it streams a reply, as a model would
}

// asked is what a scripted provider was asked with
type asked struct {
	messages []llm.Message
	tools    []string
	opts     *llm.Opts
}

var _ llm.Generator = (*scripted)(nil)

func newScripted(replies ...*providerschema.Message) *scripted {
	return &scripted{provider: &provider{name: "scripted", models: []string{"model"}}, replies: replies}
}

func (s *scripted) Self() llm.Provider { return s }

func (s *scripted) GetModel(ctx context.Context, name string) (llm.Model, error) {
	if _, err := s.provider.GetModel(ctx, name); err != nil {
		return nil, err
	}
	return &model{name: name, provider: s, input: s.input}, nil
}

func (s *scripted) ListModels(context.Context) ([]llm.Model, error) {
	var result []llm.Model
	for _, name := range s.models {
		result = append(result, &model{name: name, provider: s, input: s.input})
	}
	return result, nil
}

func (s *scripted) WithoutSession(_ context.Context, _ llm.Model, messages []llm.Message, tools []llm.Tool, stream llm.GeneratorStreamFn, opts ...llm.Opt) (llm.Message, llm.Usage, error) {
	o, err := llm.ApplyOpts(opts...)
	if err != nil {
		return nil, llm.Usage{}, err
	}
	var names []string
	for _, tool := range tools {
		names = append(names, tool.Name())
	}
	s.asked = append(s.asked, asked{messages: messages, tools: names, opts: o})
	if s.err != nil {
		return nil, llm.Usage{}, s.err
	} else if len(s.replies) == 0 {
		return nil, llm.Usage{}, errors.New("no more replies")
	}
	reply := s.replies[0]
	s.replies = s.replies[1:]
	if stream != nil {
		time.Sleep(s.delay)
		if text := reply.Text(); text != "" {
			stream(llm.StreamText, text)
		}
	}
	usage := llm.Usage{InputTokens: 10, OutputTokens: 5}
	if s.usage != nil {
		usage = *s.usage
	}
	if s.byValue {
		return *reply, usage, nil
	}
	return reply, usage, nil
}

// text returns a reply with the text
func text(value string) *providerschema.Message {
	return &providerschema.Message{MessageMeta: providerschema.MessageMeta{Sender: "assistant", Content: []providerschema.ContentBlock{{Text: types.Ptr(value)}}}}
}

// call returns a reply which calls the tool with the input
func call(name, input string) *providerschema.Message {
	return &providerschema.Message{MessageMeta: providerschema.MessageMeta{Sender: "assistant", Content: []providerschema.ContentBlock{
		{ToolCall: &providerschema.ToolCall{ID: "call-" + name, Name: name, Input: json.RawMessage(input)}},
	}}}
}

// question returns a request to ask the model the question
func question(value string) schema.AskRequest {
	return schema.AskRequest{
		Model:    types.Ptr("model"),
		Messages: []providerschema.MessageMeta{{Sender: "user", Content: []providerschema.ContentBlock{{Text: types.Ptr(value)}}}},
	}
}

// newAskRegistry returns a registry with the provider, and a toolkit with an
// echo tool, as test:echo
func newAskRegistry(t *testing.T, p llm.Provider) *registry.Registry {
	t.Helper()
	tools, err := toolkit.New(context.Background())
	require.NoError(t, err)
	require.NoError(t, tools.Add(toolkit.NewNamespace("test", "", echoTool{})))
	r, err := registry.New(registry.WithToolkit(tools))
	require.NoError(t, err)
	require.NoError(t, r.Add("", p))
	return r
}

// echoTool is a tool which returns its input
type echoTool struct{ toolkit.Base }

func (echoTool) Name() string                                              { return "echo" }
func (echoTool) Description() string                                       { return "Returns its input" }
func (echoTool) InputSchema() *jsonschema.Schema                           { return nil }
func (echoTool) Run(_ context.Context, input json.RawMessage) (any, error) { return input, nil }

///////////////////////////////////////////////////////////////////////////////
// TESTS

func Test_Ask(t *testing.T) {
	ctx := context.Background()

	t.Run("Text", func(t *testing.T) {
		// The model replies after the system prompt, with the options, to
		// the messages, and the reply has its canonical name, and usage
		p := newScripted(text("Hello"))
		r := newAskRegistry(t, p)
		req := question("Hi")
		req.System = types.Ptr("Be brief")
		req.Opt = map[string]json.RawMessage{"temperature": json.RawMessage(`0.5`)}
		response, err := r.Ask(ctx, req, nil)
		require.NoError(t, err)
		assert.Equal(t, "scripted:model", response.Model)
		assert.Equal(t, "Hello", response.Message.Text())
		assert.False(t, response.Schema)
		assert.Empty(t, response.Result)
		assert.Equal(t, llm.Usage{InputTokens: 10, OutputTokens: 5}, withoutDuration(response.Usage))
		require.Len(t, p.asked, 1)
		assert.Equal(t, "Hi", p.asked[0].messages[0].Text())
		system, _ := llm.GetOpt[string](p.asked[0].opts, llm.SystemPromptKey)
		assert.Equal(t, "Be brief", system)
		temperature, _ := llm.GetOpt[float64](p.asked[0].opts, llm.TemperatureKey)
		assert.Equal(t, 0.5, temperature)
	})

	t.Run("Tools", func(t *testing.T) {
		// The model can call the tools, whose calls are returned, rather than
		// run
		p := newScripted(call("test__echo", `{"city":"Berlin"}`))
		r := newAskRegistry(t, p)
		req := question("Weather?")
		req.Tools = []string{"test:echo"}
		response, err := r.Ask(ctx, req, nil)
		require.NoError(t, err)
		assert.Equal(t, []string{"test__echo"}, p.asked[0].tools)
		if calls := response.Message.ToolCalls(); assert.Len(t, calls, 1) {
			assert.Equal(t, "test__echo", calls[0].Name)
			assert.JSONEq(t, `{"city":"Berlin"}`, string(calls[0].Input))
		}
	})

	t.Run("Schema", func(t *testing.T) {
		// With a schema, the model replies by calling the respond tool,
		// which the system prompt says, whose result is the reply's
		p := newScripted(call(toolkit.RespondName, `{"result":{"city":"Berlin"}}`))
		r := newAskRegistry(t, p)
		req := question("Where?")
		req.System = types.Ptr("Be brief")
		req.Schema = json.RawMessage(`{"type":"object","properties":{"city":{"type":"string"}},"required":["city"]}`)
		response, err := r.Ask(ctx, req, nil)
		require.NoError(t, err)
		assert.True(t, response.Schema)
		assert.JSONEq(t, `{"city":"Berlin"}`, string(response.Result))
		assert.JSONEq(t, `{"city":"Berlin"}`, response.Message.Text())
		assert.Empty(t, response.Message.ToolCalls())
		assert.Equal(t, []string{toolkit.RespondName}, p.asked[0].tools)
		system, _ := llm.GetOpt[string](p.asked[0].opts, llm.SystemPromptKey)
		assert.Equal(t, "Be brief\n\n"+toolkit.RespondPrompt, system)
	})

	t.Run("SchemaNudge", func(t *testing.T) {
		// A model which replies with text is asked once to call the respond
		// tool, and the usage is of both
		p := newScripted(text("Berlin"), call(toolkit.RespondName, `{"result":"Berlin"}`))
		r := newAskRegistry(t, p)
		req := question("Where?")
		req.Schema = json.RawMessage(`{"type":"string"}`)
		response, err := r.Ask(ctx, req, nil)
		require.NoError(t, err)
		assert.JSONEq(t, `"Berlin"`, string(response.Result))
		assert.Equal(t, llm.Usage{InputTokens: 20, OutputTokens: 10}, withoutDuration(response.Usage))
		require.Len(t, p.asked, 2)
		second := p.asked[1].messages
		require.Len(t, second, 3)
		assert.Equal(t, "Berlin", second[1].Text())
		assert.Equal(t, toolkit.RespondNudge, second[2].Text())
	})

	t.Run("SchemaRetry", func(t *testing.T) {
		// A result which doesn't conform is returned to the model, which is
		// asked again
		p := newScripted(call(toolkit.RespondName, `{"result":1}`), call(toolkit.RespondName, `{"result":"Berlin"}`))
		r := newAskRegistry(t, p)
		req := question("Where?")
		req.Schema = json.RawMessage(`{"type":"string"}`)
		response, err := r.Ask(ctx, req, nil)
		require.NoError(t, err)
		assert.JSONEq(t, `"Berlin"`, string(response.Result))
		require.Len(t, p.asked, 2)
		if results := p.asked[1].messages[2].ToolResults(); assert.Len(t, results, 1) {
			assert.True(t, results[0].IsError)
			assert.Contains(t, string(results[0].Content), "schema")
		}
	})

	t.Run("SchemaExhausted", func(t *testing.T) {
		// A model which doesn't reply with a result which conforms, when
		// it's been asked as many times as it can be, has no result
		p := newScripted(call(toolkit.RespondName, `{"result":1}`), call(toolkit.RespondName, `{"result":2}`), call(toolkit.RespondName, `{"result":3}`))
		r := newAskRegistry(t, p)
		req := question("Where?")
		req.Schema = json.RawMessage(`{"type":"string"}`)
		response, err := r.Ask(ctx, req, nil)
		require.NoError(t, err)
		assert.True(t, response.Schema)
		assert.Empty(t, response.Result)
		assert.Len(t, p.asked, 3)

		// The message is the model's last reply, rather than the results of
		// the tools it called, without its call, as it wasn't run
		assert.Equal(t, "assistant", response.Message.Role())
		assert.Equal(t, providerschema.ResultMaxIterations, response.Message.Result)
		assert.Empty(t, response.Message.ToolCalls())
	})

	t.Run("SchemaText", func(t *testing.T) {
		// A model which keeps replying with text is asked to call the respond
		// tool until it's been asked as many times as it can be, which is the
		// max_iterations option, when it's set, and has no result
		p := newScripted(text("Berlin"), text("Still Berlin"))
		r := newAskRegistry(t, p)
		req := question("Where?")
		req.Schema = json.RawMessage(`{"type":"string"}`)
		req.Opt = map[string]json.RawMessage{llm.MaxIterationsKey: json.RawMessage(`2`)}
		response, err := r.Ask(ctx, req, nil)
		require.NoError(t, err)
		assert.Empty(t, response.Result)
		assert.Equal(t, "Still Berlin", response.Message.Text())
		assert.Equal(t, providerschema.ResultMaxIterations, response.Message.Result)
		assert.Len(t, p.asked, 2)
		assert.False(t, p.asked[0].opts.Has(llm.MaxIterationsKey))
	})

	t.Run("SchemaTools", func(t *testing.T) {
		// With a schema, the model can call tools, whose calls are returned,
		// rather than run, without the respond tool's, if it called it too,
		// as it's to call it once it has their results
		reply := call("test__echo", `{"city":"Berlin"}`)
		reply.Content = append(reply.Content, call(toolkit.RespondName, `{"result":"Berlin"}`).Content...)
		p := newScripted(reply)
		r := newAskRegistry(t, p)
		req := question("Where?")
		req.Tools = []string{"test:echo"}
		req.Schema = json.RawMessage(`{"type":"string"}`)
		response, err := r.Ask(ctx, req, nil)
		require.NoError(t, err)
		assert.Empty(t, response.Result)
		assert.Equal(t, providerschema.ResultToolCall, response.Message.Result)
		if calls := response.Message.ToolCalls(); assert.Len(t, calls, 1) {
			assert.Equal(t, "test__echo", calls[0].Name)
		}
		assert.Len(t, p.asked, 1)
		assert.Equal(t, []string{"test__echo", toolkit.RespondName}, p.asked[0].tools)
	})

	t.Run("Compact", func(t *testing.T) {
		// The messages are compacted to fit the model's context, so the result
		// of a tool of an earlier turn is removed from what the model is sent
		p := newScripted(text("Done"))
		r := newAskRegistry(t, p)
		req := question("Second")
		req.Messages = append([]providerschema.MessageMeta{
			{Sender: "user", Content: []providerschema.ContentBlock{{Text: types.Ptr("First")}}},
			{Sender: "assistant", Content: []providerschema.ContentBlock{{ToolCall: &providerschema.ToolCall{ID: "old", Name: "test__echo"}}}},
			{Sender: "tool", Content: []providerschema.ContentBlock{providerschema.NewToolResult("old", "test__echo", strings.Repeat("a", 20_000))}},
		}, req.Messages...)
		req.Opt = map[string]json.RawMessage{llm.ContextWindowKey: json.RawMessage(`4000`)}
		response, err := r.Ask(ctx, req, nil)
		require.NoError(t, err)
		assert.Equal(t, "Done", response.Message.Text())
		require.Len(t, p.asked, 1)
		require.Len(t, p.asked[0].messages, 4)
		assert.Contains(t, string(p.asked[0].messages[2].ToolResults()[0].Content), "was removed")
		assert.Greater(t, len(req.Messages[2].Content[0].ToolResult.Content), 20_000)
	})

	t.Run("MaxContext", func(t *testing.T) {
		// The latest message doesn't fit in the model's context, so the model
		// isn't asked, and the reply is empty, with max_context as its result,
		// and no result, with a schema
		p := newScripted(text("Never"))
		r := newAskRegistry(t, p)
		req := question(strings.Repeat("a", 20_000))
		req.Schema = json.RawMessage(`{"type":"string"}`)
		req.Opt = map[string]json.RawMessage{llm.ContextWindowKey: json.RawMessage(`4000`)}
		response, err := r.Ask(ctx, req, nil)
		require.NoError(t, err)
		assert.Empty(t, p.asked)
		assert.Equal(t, providerschema.ResultMaxContext, response.Message.Result)
		assert.Empty(t, response.Message.Text())
		assert.Empty(t, response.Result)
	})

	t.Run("Value", func(t *testing.T) {
		// A reply which is a message by value is kept whole, with its
		// thinking
		p := newScripted(&providerschema.Message{MessageMeta: providerschema.MessageMeta{Sender: "assistant", Content: []providerschema.ContentBlock{
			{Thinking: types.Ptr("Hmm")},
			{Text: types.Ptr("Hello")},
		}}})
		p.byValue = true
		response, err := newAskRegistry(t, p).Ask(ctx, question("Hi"), nil)
		require.NoError(t, err)
		if assert.Len(t, response.Message.Content, 2) {
			assert.Equal(t, "Hmm", types.Value(response.Message.Content[0].Thinking))
		}
		assert.Equal(t, "Hello", response.Message.Text())
	})

	t.Run("ToolChoice", func(t *testing.T) {
		// A tool choice of none has no tools, and isn't sent
		p := newScripted(text("Hello"))
		r := newAskRegistry(t, p)
		req := question("Hi")
		req.Tools = []string{"test:echo"}
		req.Opt = map[string]json.RawMessage{"tool_choice": json.RawMessage(`"none"`)}
		_, err := r.Ask(ctx, req, nil)
		require.NoError(t, err)
		assert.Empty(t, p.asked[0].tools)
		assert.False(t, p.asked[0].opts.Has(llm.ToolChoiceKey))

		// But a schema needs tools, and another choice needs tools
		req.Schema = json.RawMessage(`{"type":"string"}`)
		_, err = r.Ask(ctx, req, nil)
		assert.ErrorIs(t, err, llm.ErrBadParameter)
		req = question("Hi")
		req.Opt = map[string]json.RawMessage{"tool_choice": json.RawMessage(`"any"`)}
		_, err = r.Ask(ctx, req, nil)
		assert.ErrorIs(t, err, llm.ErrBadParameter)
	})

	t.Run("Stream", func(t *testing.T) {
		// Each reply is streamed, separated by a blank line
		p := newScripted(text("Berlin"), call(toolkit.RespondName, `{"result":"Berlin"}`))
		r := newAskRegistry(t, p)
		req := question("Where?")
		req.Schema = json.RawMessage(`{"type":"string"}`)
		var streamed strings.Builder
		_, err := r.Ask(ctx, req, func(_ llm.StreamType, text string) { streamed.WriteString(text) })
		require.NoError(t, err)
		assert.Equal(t, "Berlin", streamed.String())

		p = newScripted(text("One"), text("Two"), text("Three"))
		r = newAskRegistry(t, p)
		streamed.Reset()
		_, err = r.Ask(ctx, req, func(_ llm.StreamType, text string) { streamed.WriteString(text) })
		require.NoError(t, err)
		assert.Equal(t, "One\n\nTwo\n\nThree", streamed.String())
	})

	t.Run("Errors", func(t *testing.T) {
		p := newScripted(text("Hello"))
		r := newAskRegistry(t, p)

		// A request needs a model, which exists, and messages
		_, err := r.Ask(ctx, schema.AskRequest{Messages: question("Hi").Messages}, nil)
		assert.ErrorIs(t, err, llm.ErrBadParameter)
		_, err = r.Ask(ctx, schema.AskRequest{Model: types.Ptr("model")}, nil)
		assert.ErrorIs(t, err, llm.ErrBadParameter)
		req := question("Hi")
		req.Model = types.Ptr("missing")
		_, err = r.Ask(ctx, req, nil)
		assert.ErrorIs(t, err, llm.ErrNotFound)

		// And tools which exist, and a schema which is a schema
		req = question("Hi")
		req.Tools = []string{"test:missing"}
		_, err = r.Ask(ctx, req, nil)
		assert.ErrorIs(t, err, llm.ErrNotFound)
		req = question("Hi")
		req.Schema = json.RawMessage(`{"type":1}`)
		_, err = r.Ask(ctx, req, nil)
		assert.ErrorIs(t, err, llm.ErrBadParameter)

		// And the system prompt and schema as fields, rather than options
		for _, key := range []string{llm.SystemPromptKey, llm.JSONSchemaKey} {
			req = question("Hi")
			req.Opt = map[string]json.RawMessage{key: json.RawMessage(`"x"`)}
			_, err = r.Ask(ctx, req, nil)
			assert.ErrorIs(t, err, llm.ErrBadParameter, key)
		}

		// A registry without a toolkit has no tools
		plain, err := registry.New()
		require.NoError(t, err)
		require.NoError(t, plain.Add("", newScripted(text("Hello"))))
		req = question("Hi")
		req.Tools = []string{"test:echo"}
		_, err = plain.Ask(ctx, req, nil)
		assert.ErrorIs(t, err, llm.ErrNotFound)

		// And a provider which fails is unavailable
		p.err = errors.New("connection refused")
		_, err = r.Ask(ctx, question("Hi"), nil)
		assert.ErrorIs(t, err, llm.ErrServiceUnavailable)
	})
}

// withoutDuration returns the usage, without how long it took, which varies
func withoutDuration(usage llm.Usage) llm.Usage {
	usage.Duration = 0
	return usage
}
