package registry_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

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

// scripted is a provider which generates its replies in turn, and records
// what it was asked with
type scripted struct {
	*provider
	replies []*providerschema.Message
	err     error
	asked   []asked
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
	return &model{name: name, provider: s}, nil
}

func (s *scripted) ListModels(context.Context) ([]llm.Model, error) {
	var result []llm.Model
	for _, name := range s.models {
		result = append(result, &model{name: name, provider: s})
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
		if text := reply.Text(); text != "" {
			stream(llm.StreamText, text)
		}
	}
	return reply, llm.Usage{InputTokens: 10, OutputTokens: 5}, nil
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
		Model:    "model",
		Messages: []providerschema.MessageMeta{{Sender: "user", Content: []providerschema.ContentBlock{{Text: types.Ptr(value)}}}},
	}
}

// newAskRegistry returns a registry with the provider, and a toolkit with an
// echo tool, as test:echo
func newAskRegistry(t *testing.T, p llm.Provider) *registry.Registry {
	t.Helper()
	tools, err := toolkit.New(context.Background())
	require.NoError(t, err)
	require.NoError(t, tools.Add("test", echoTool{}))
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
		req.System = "Be brief"
		req.Opt = map[string]json.RawMessage{"temperature": json.RawMessage(`0.5`)}
		response, err := r.Ask(ctx, req, nil)
		require.NoError(t, err)
		assert.Equal(t, "scripted:model", response.Model)
		assert.Equal(t, "Hello", response.Message.Text())
		assert.False(t, response.Schema)
		assert.Empty(t, response.Result)
		assert.Equal(t, llm.Usage{InputTokens: 10, OutputTokens: 5}, response.Usage)
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
		p := newScripted(call("test_echo", `{"city":"Berlin"}`))
		r := newAskRegistry(t, p)
		req := question("Weather?")
		req.Tools = []string{"test:echo"}
		response, err := r.Ask(ctx, req, nil)
		require.NoError(t, err)
		assert.Equal(t, []string{"test_echo"}, p.asked[0].tools)
		if calls := response.Message.ToolCalls(); assert.Len(t, calls, 1) {
			assert.Equal(t, "test_echo", calls[0].Name)
			assert.JSONEq(t, `{"city":"Berlin"}`, string(calls[0].Input))
		}
	})

	t.Run("Schema", func(t *testing.T) {
		// With a schema, the model replies by calling the respond tool,
		// which the system prompt says, whose result is the reply's
		p := newScripted(call(toolkit.RespondName, `{"result":{"city":"Berlin"}}`))
		r := newAskRegistry(t, p)
		req := question("Where?")
		req.System = "Be brief"
		req.Schema = json.RawMessage(`{"type":"object","properties":{"city":{"type":"string"}},"required":["city"]}`)
		response, err := r.Ask(ctx, req, nil)
		require.NoError(t, err)
		assert.True(t, response.Schema)
		assert.JSONEq(t, `{"city":"Berlin"}`, string(response.Result))
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
		assert.Equal(t, llm.Usage{InputTokens: 20, OutputTokens: 10}, response.Usage)
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

		p = newScripted(text("One"), text("Two"))
		r = newAskRegistry(t, p)
		streamed.Reset()
		_, err = r.Ask(ctx, req, func(_ llm.StreamType, text string) { streamed.WriteString(text) })
		require.NoError(t, err)
		assert.Equal(t, "One\n\nTwo", streamed.String())
	})

	t.Run("Errors", func(t *testing.T) {
		p := newScripted(text("Hello"))
		r := newAskRegistry(t, p)

		// A request needs a model, which exists, and messages
		_, err := r.Ask(ctx, schema.AskRequest{Messages: question("Hi").Messages}, nil)
		assert.ErrorIs(t, err, llm.ErrBadParameter)
		_, err = r.Ask(ctx, schema.AskRequest{Model: "model"}, nil)
		assert.ErrorIs(t, err, llm.ErrBadParameter)
		req := question("Hi")
		req.Model = "missing"
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
