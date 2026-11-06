package registry_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	eventschema "github.com/mutablelogic/go-llm/broadcaster/schema"
	providerschema "github.com/mutablelogic/go-llm/provider/schema"
	registry "github.com/mutablelogic/go-llm/registry"
	toolkit "github.com/mutablelogic/go-llm/toolkit"
	jsonschema "github.com/mutablelogic/go-server/pkg/jsonschema"
	types "github.com/mutablelogic/go-server/pkg/types"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
	attribute "go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	tracetest "go.opentelemetry.io/otel/sdk/trace/tracetest"
)

///////////////////////////////////////////////////////////////////////////////
// FAKES

// failTool is a tool which fails
type failTool struct{ echoTool }

func (failTool) Name() string                    { return "fail" }
func (failTool) InputSchema() *jsonschema.Schema { return nil }
func (failTool) Run(context.Context, json.RawMessage) (any, error) {
	return nil, errors.New("it failed")
}

// truncated returns a reply which calls the tool, and was truncated
func truncated(name, input string) *providerschema.Message {
	reply := call(name, input)
	reply.Result = providerschema.ResultMaxTokens
	return reply
}

// assertAnswered asserts that each tool call in the messages is followed by
// its result, in the next message, so they can be sent to a model again
func assertAnswered(t *testing.T, messages []providerschema.Message) {
	t.Helper()
	for i, message := range messages {
		for _, call := range message.ToolCalls() {
			if !assert.Less(t, i+1, len(messages), "tool call %q has no result", call.ID) {
				continue
			}
			assert.True(t, slices.ContainsFunc(messages[i+1].ToolResults(), func(result llm.ToolResult) bool {
				return result.ID == call.ID
			}), "tool call %q has no result", call.ID)
		}
	}
}

// newAgentLoop returns the registry with the provider, and its model
func newAgentLoop(t *testing.T, p *scripted) (*registry.Registry, llm.Model) {
	t.Helper()
	r := newAskRegistry(t, p)
	model, err := r.GetModel(context.Background(), "model")
	require.NoError(t, err)
	return r, model
}

///////////////////////////////////////////////////////////////////////////////
// TESTS

func Test_AgentLoop(t *testing.T) {
	ctx := context.Background()
	tools := []llm.Tool{echoTool{}, failTool{}}

	t.Run("NoToolCalls", func(t *testing.T) {
		// The model replies without calling a tool, so it's asked once
		p := newScripted(text("Hello"))
		r, model := newAgentLoop(t, p)
		added, usage, err := r.AgentLoop(ctx, model, []llm.Message{user(t, "Hi")}, tools, nil)
		require.NoError(t, err)
		require.Len(t, added, 1)
		assert.Equal(t, "Hello", added[0].Text())
		assert.Equal(t, providerschema.ResultStop, added[0].Result)
		assert.Equal(t, llm.Usage{InputTokens: 10, OutputTokens: 5}, withoutDuration(usage))
		assert.NotZero(t, usage.Duration, "how long the loop took")
		assert.Len(t, p.asked, 1)
		assert.Equal(t, []string{"echo", "fail"}, p.asked[0].tools)
	})

	t.Run("ToolCalls", func(t *testing.T) {
		// The model calls a tool, which is run, and is asked again with its
		// result, after which it replies, and the caller's messages are kept
		p := newScripted(call("echo", `{"city":"Berlin"}`), text("Sunny"))
		r, model := newAgentLoop(t, p)
		messages := []llm.Message{user(t, "Weather?")}
		added, usage, err := r.AgentLoop(ctx, model, messages, tools, nil)
		require.NoError(t, err)
		require.Len(t, added, 3)
		assert.Len(t, added[0].ToolCalls(), 1)
		assert.Equal(t, "tool", added[1].Role())
		if results := added[1].ToolResults(); assert.Len(t, results, 1) {
			assert.Equal(t, "call-echo", results[0].ID)
			assert.False(t, results[0].IsError)
			assert.JSONEq(t, `{"city":"Berlin"}`, string(results[0].Content))
		}
		assert.Equal(t, "Sunny", added[2].Text())
		assert.Equal(t, providerschema.ResultStop, added[2].Result)
		assert.Equal(t, llm.Usage{InputTokens: 20, OutputTokens: 10}, withoutDuration(usage))
		require.Len(t, p.asked, 2)
		assert.Len(t, p.asked[0].messages, 1)
		if assert.Len(t, p.asked[1].messages, 3) {
			assert.Equal(t, "tool", p.asked[1].messages[2].Role())
		}
		assert.Len(t, messages, 1)
	})

	t.Run("ToolErrors", func(t *testing.T) {
		// A tool which fails, or isn't one of the tools, has its error as its
		// result, and the model is asked again
		p := newScripted(
			&providerschema.Message{MessageMeta: providerschema.MessageMeta{Sender: "assistant", Content: []providerschema.ContentBlock{
				{ToolCall: &providerschema.ToolCall{ID: "1", Name: "fail"}},
				{ToolCall: &providerschema.ToolCall{ID: "2", Name: "missing"}},
			}}},
			text("Sorry"),
		)
		r, model := newAgentLoop(t, p)
		added, _, err := r.AgentLoop(ctx, model, []llm.Message{user(t, "Go")}, tools, nil)
		require.NoError(t, err)
		require.Len(t, added, 3)
		if results := added[1].ToolResults(); assert.Len(t, results, 2) {
			assert.True(t, results[0].IsError)
			assert.Contains(t, string(results[0].Content), "it failed")
			assert.True(t, results[1].IsError)
			assert.Contains(t, string(results[1].Content), "missing")
		}
		assert.Equal(t, providerschema.ResultStop, added[2].Result)
	})

	t.Run("MaxTokens", func(t *testing.T) {
		// The model's reply was truncated, so the tools it called, whose
		// input is incomplete, aren't run, and are left out of it, but its
		// text is kept
		reply := truncated("echo", `{"city":`)
		reply.Content = append([]providerschema.ContentBlock{{Text: types.Ptr("Checking")}}, reply.Content...)
		p := newScripted(reply)
		r, model := newAgentLoop(t, p)
		added, _, err := r.AgentLoop(ctx, model, []llm.Message{user(t, "Weather?")}, tools, nil)
		require.NoError(t, err)
		require.Len(t, added, 1)
		assert.Equal(t, providerschema.ResultMaxTokens, added[0].Result)
		assert.Equal(t, "Checking", added[0].Text())
		assert.Empty(t, added[0].ToolCalls())
		assert.Len(t, p.asked, 1)
		assertAnswered(t, added)

		// The provider's reply isn't changed
		assert.Len(t, reply.ToolCalls(), 1)
	})

	t.Run("Unfinished", func(t *testing.T) {
		// A reply which was blocked, or ended in an error, stops the loop, and
		// the tools it called aren't run, and are left out of it, even when
		// the reply has a schema, so isn't asked to call the respond tool
		for _, result := range []providerschema.ResultType{providerschema.ResultBlocked, providerschema.ResultError} {
			for name, opts := range map[string][]llm.Opt{
				"Text":   nil,
				"Schema": {llm.WithJSONSchema(json.RawMessage(`{"type":"string"}`))},
			} {
				t.Run(result.String()+name, func(t *testing.T) {
					reply := call("echo", `{}`)
					reply.Content = append([]providerschema.ContentBlock{{Text: types.Ptr("Partial")}}, reply.Content...)
					reply.Result = result
					p := newScripted(reply, text("Never"))
					r, model := newAgentLoop(t, p)
					added, _, err := r.AgentLoop(ctx, model, []llm.Message{user(t, "Go")}, tools, nil, opts...)
					require.NoError(t, err)
					require.Len(t, added, 1)
					assert.Equal(t, result, added[0].Result)
					assert.Equal(t, "Partial", added[0].Text())
					assert.Empty(t, added[0].ToolCalls())
					assert.Len(t, p.asked, 1)
				})
			}
		}
	})

	t.Run("MaxTokensOnlyToolCalls", func(t *testing.T) {
		// A truncated reply which only called tools has empty text
		p := newScripted(truncated("echo", `{"city":`))
		r, model := newAgentLoop(t, p)
		added, _, err := r.AgentLoop(ctx, model, []llm.Message{user(t, "Weather?")}, tools, nil)
		require.NoError(t, err)
		require.Len(t, added, 1)
		assert.Equal(t, providerschema.ResultMaxTokens, added[0].Result)
		assert.Empty(t, added[0].ToolCalls())
		if assert.Len(t, added[0].Content, 1) {
			assert.Equal(t, "", types.Value(added[0].Content[0].Text))
		}
	})

	t.Run("MaxIterations", func(t *testing.T) {
		// The model calls tools each time it's asked, so it's asked as many
		// times as it can be, and the tools of its last reply aren't run, so
		// are left out of it, which is the last message
		p := newScripted(call("echo", `{}`), call("echo", `{}`), text("Never"))
		r, model := newAgentLoop(t, p)
		added, _, err := r.AgentLoop(ctx, model, []llm.Message{user(t, "Loop")}, tools, nil, llm.WithMaxIterations(2), llm.WithTemperature(0.5))
		require.NoError(t, err)
		require.Len(t, added, 3)
		assert.Equal(t, "tool", added[1].Role())
		assert.Equal(t, "assistant", added[2].Role())
		assert.Empty(t, added[2].ToolCalls())
		assert.Equal(t, providerschema.ResultMaxIterations, added[2].Result)
		require.Len(t, p.asked, 2)
		assertAnswered(t, added)

		// The option is the loop's, so isn't passed to the model, unlike the
		// other options
		for _, asked := range p.asked {
			assert.False(t, asked.opts.Has(llm.MaxIterationsKey))
			assert.True(t, asked.opts.Has(llm.TemperatureKey))
		}
	})

	t.Run("InvalidMaxIterations", func(t *testing.T) {
		// The most iterations is one or more, so the model isn't asked
		for name, opt := range map[string]llm.Opt{
			"Zero":   llm.WithOpt(llm.MaxIterationsKey, uint(0)),
			"String": llm.WithOpt(llm.MaxIterationsKey, "ten"),
		} {
			t.Run(name, func(t *testing.T) {
				p := newScripted(text("Never"))
				r, model := newAgentLoop(t, p)
				_, _, err := r.AgentLoop(ctx, model, []llm.Message{user(t, "Hi")}, tools, nil, opt)
				assert.ErrorIs(t, err, llm.ErrBadParameter)
				assert.Empty(t, p.asked)
			})
		}
	})

	t.Run("ToolResultLimit", func(t *testing.T) {
		// A tool's result is truncated to the most tokens of a result, as the
		// model's provider measures them, which is an estimate when it
		// doesn't, before it's sent to the model
		p := newScripted(call("big", `{}`), text("Done"))
		r, model := newAgentLoop(t, p)
		added, _, err := r.AgentLoop(ctx, model, []llm.Message{user(t, "Go")}, []llm.Tool{bigTool{}}, nil)
		require.NoError(t, err)
		require.Len(t, added, 3)
		results := added[1].ToolResults()
		require.Len(t, results, 1)
		assert.False(t, results[0].IsError)
		assert.True(t, json.Valid(results[0].Content))
		assert.LessOrEqual(t, len(results[0].Content), 4*registry.DefaultToolResultLimit)
		assert.Greater(t, len(results[0].Content), 3*registry.DefaultToolResultLimit)
		assert.Contains(t, string(results[0].Content), "more characters]")
		assert.Equal(t, string(results[0].Content), string(p.asked[1].messages[2].ToolResults()[0].Content))
	})

	t.Run("ToolTimeout", func(t *testing.T) {
		// A tool which doesn't return in time has its result that it timed
		// out, and the model is asked again, without the option, which is
		// the loop's
		release := make(chan struct{})
		defer close(release)
		p := newScripted(call("hang", `{}`), text("Sorry"))
		r, model := newAgentLoop(t, p)
		added, _, err := r.AgentLoop(ctx, model, []llm.Message{user(t, "Go")}, []llm.Tool{hangTool{release: release}}, nil, llm.WithOpt(llm.ToolTimeoutKey, "50ms"))
		require.NoError(t, err)
		require.Len(t, added, 3)
		if results := added[1].ToolResults(); assert.Len(t, results, 1) {
			assert.True(t, results[0].IsError)
			assert.Contains(t, string(results[0].Content), "timed out after 50ms")
		}
		assert.Equal(t, providerschema.ResultStop, added[2].Result)
		for _, asked := range p.asked {
			assert.False(t, asked.opts.Has(llm.ToolTimeoutKey))
		}
	})

	t.Run("InvalidToolTimeout", func(t *testing.T) {
		// The most time is a duration, or a string such as "30s", which is
		// more than zero, rather than a number, so the model isn't asked
		for name, opt := range map[string]llm.Opt{
			"Number":   llm.WithOpt(llm.ToolTimeoutKey, json.RawMessage(`30`)),
			"String":   llm.WithOpt(llm.ToolTimeoutKey, "soon"),
			"Zero":     llm.WithOpt(llm.ToolTimeoutKey, "0s"),
			"Negative": llm.WithOpt(llm.ToolTimeoutKey, -time.Second),
		} {
			t.Run(name, func(t *testing.T) {
				p := newScripted(text("Never"))
				r, model := newAgentLoop(t, p)
				_, _, err := r.AgentLoop(ctx, model, []llm.Message{user(t, "Hi")}, tools, nil, opt)
				assert.ErrorIs(t, err, llm.ErrBadParameter)
				assert.Empty(t, p.asked)
			})
		}
	})

	t.Run("DefaultMaxIterations", func(t *testing.T) {
		// The most iterations is the default, when it isn't set
		var replies []*providerschema.Message
		for range registry.DefaultMaxIterations + 1 {
			replies = append(replies, call("echo", `{}`))
		}
		p := newScripted(replies...)
		r, model := newAgentLoop(t, p)
		added, _, err := r.AgentLoop(ctx, model, []llm.Message{user(t, "Loop")}, tools, nil)
		require.NoError(t, err)
		assert.Len(t, p.asked, registry.DefaultMaxIterations)
		require.Len(t, added, 2*registry.DefaultMaxIterations-1)
		assert.Equal(t, providerschema.ResultMaxIterations, added[len(added)-1].Result)
		assertAnswered(t, added)
	})

	t.Run("Stream", func(t *testing.T) {
		// Each reply is streamed, separated by a blank line
		p := newScripted(call("echo", `{}`), text("One"), text("Two"))
		p.replies[0].Content = append(p.replies[0].Content, providerschema.ContentBlock{Text: types.Ptr("Calling")})
		r, model := newAgentLoop(t, p)
		var streamed string
		_, _, err := r.AgentLoop(ctx, model, []llm.Message{user(t, "Go")}, tools, func(_ llm.StreamType, text string) { streamed += text })
		require.NoError(t, err)
		assert.Equal(t, "Calling\n\nOne", streamed)
	})

	t.Run("Error", func(t *testing.T) {
		// The provider's error is returned
		p := newScripted()
		p.err = errors.New("unavailable")
		r, model := newAgentLoop(t, p)
		_, _, err := r.AgentLoop(ctx, model, []llm.Message{user(t, "Hi")}, tools, nil)
		assert.Error(t, err)
	})

	t.Run("Cancelled", func(t *testing.T) {
		// The context's error is returned when it's done while a tool runs
		ctx, cancel := context.WithCancel(ctx)
		p := newScripted(call("cancel", `{}`), text("Never"))
		r, model := newAgentLoop(t, p)
		_, _, err := r.AgentLoop(ctx, model, []llm.Message{user(t, "Go")}, []llm.Tool{cancelTool{cancel: cancel}}, nil)
		assert.ErrorIs(t, err, context.Canceled)
		assert.Len(t, p.asked, 1)
	})
}

// cancelTool is a tool which cancels the context it runs in
type cancelTool struct {
	echoTool
	cancel context.CancelFunc
}

func (cancelTool) Name() string { return "cancel" }
func (t cancelTool) Run(context.Context, json.RawMessage) (any, error) {
	t.cancel()
	return nil, nil
}

func Test_AgentLoopSchema(t *testing.T) {
	ctx := context.Background()
	tools := []llm.Tool{echoTool{}}
	schema := llm.WithJSONSchema(json.RawMessage(`{"type":"object","properties":{"city":{"type":"string"}},"required":["city"]}`))

	t.Run("Result", func(t *testing.T) {
		// The model calls the respond tool with a result which conforms, so
		// the reply is the result, as JSON text, rather than the call
		p := newScripted(call(toolkit.RespondName, `{"result":{"city":"Berlin"}}`))
		r, model := newAgentLoop(t, p)
		added, _, err := r.AgentLoop(ctx, model, []llm.Message{user(t, "Where?")}, tools, nil, schema, llm.WithSystemPrompt("Be brief"))
		require.NoError(t, err)
		require.Len(t, added, 1)
		assert.Equal(t, providerschema.ResultStop, added[0].Result)
		assert.JSONEq(t, `{"city":"Berlin"}`, added[0].Text())
		assert.Empty(t, added[0].ToolCalls())
		assertAnswered(t, added)

		// The model is asked with the respond tool, and the instruction to
		// call it in the system prompt, but not the schema, as an option
		require.Len(t, p.asked, 1)
		assert.Equal(t, []string{"echo", toolkit.RespondName}, p.asked[0].tools)
		system, _ := llm.GetOpt[string](p.asked[0].opts, llm.SystemPromptKey)
		assert.Equal(t, "Be brief\n\n"+toolkit.RespondPrompt, system)
		assert.False(t, p.asked[0].opts.Has(llm.JSONSchemaKey))
	})

	t.Run("Thinking", func(t *testing.T) {
		// The reply keeps its thinking, but not its text
		reply := call(toolkit.RespondName, `{"result":{"city":"Berlin"}}`)
		reply.Content = append([]providerschema.ContentBlock{{Thinking: types.Ptr("Hmm")}, {Text: types.Ptr("Here")}}, reply.Content...)
		p := newScripted(reply)
		r, model := newAgentLoop(t, p)
		added, _, err := r.AgentLoop(ctx, model, []llm.Message{user(t, "Where?")}, tools, nil, schema)
		require.NoError(t, err)
		require.Len(t, added, 1)
		if assert.Len(t, added[0].Content, 2) {
			assert.Equal(t, "Hmm", types.Value(added[0].Content[0].Thinking))
		}
		assert.JSONEq(t, `{"city":"Berlin"}`, added[0].Text())
		system, _ := llm.GetOpt[string](p.asked[0].opts, llm.SystemPromptKey)
		assert.Equal(t, toolkit.RespondPrompt, system)
	})

	t.Run("Tools", func(t *testing.T) {
		// The model calls other tools before it replies
		p := newScripted(call("echo", `{"city":"Berlin"}`), call(toolkit.RespondName, `{"result":{"city":"Berlin"}}`))
		r, model := newAgentLoop(t, p)
		added, _, err := r.AgentLoop(ctx, model, []llm.Message{user(t, "Where?")}, tools, nil, schema)
		require.NoError(t, err)
		require.Len(t, added, 3)
		assert.Equal(t, "tool", added[1].Role())
		assert.JSONEq(t, `{"city":"Berlin"}`, added[2].Text())
		assertAnswered(t, added)
	})

	t.Run("Text", func(t *testing.T) {
		// The model replies with text, so it's asked to call the respond tool
		p := newScripted(text("Berlin"), call(toolkit.RespondName, `{"result":{"city":"Berlin"}}`))
		r, model := newAgentLoop(t, p)
		added, _, err := r.AgentLoop(ctx, model, []llm.Message{user(t, "Where?")}, tools, nil, schema)
		require.NoError(t, err)
		require.Len(t, added, 3)
		assert.Equal(t, "Berlin", added[0].Text())
		assert.Equal(t, "user", added[1].Role())
		assert.Equal(t, toolkit.RespondNudge, added[1].Text())
		assert.Equal(t, providerschema.ResultStop, added[2].Result)
		assert.JSONEq(t, `{"city":"Berlin"}`, added[2].Text())
		assert.Len(t, p.asked, 2)
	})

	t.Run("TextResult", func(t *testing.T) {
		// The model replies with text which is what the respond tool would
		// have been called with, or its result, which conforms, apart from a
		// fence around it, so it's the reply, without asking again
		for name, reply := range map[string]string{
			"Input":         `{"result":{"city":"Berlin"}}`,
			"Result":        `{"city":"Berlin"}`,
			"Fenced":        "```json\n{\"result\":{\"city\":\"Berlin\"}}\n```",
			"FencedNoLang":  "```\n{\"city\":\"Berlin\"}\n```",
			"FencedOneLine": "```{\"city\":\"Berlin\"}```",
		} {
			t.Run(name, func(t *testing.T) {
				p := newScripted(text(reply))
				r, model := newAgentLoop(t, p)
				added, _, err := r.AgentLoop(ctx, model, []llm.Message{user(t, "Where?")}, tools, nil, schema)
				require.NoError(t, err)
				require.Len(t, added, 1)
				assert.Equal(t, providerschema.ResultStop, added[0].Result)
				assert.JSONEq(t, `{"city":"Berlin"}`, added[0].Text())
				assert.Len(t, p.asked, 1, "it isn't asked again")
			})
		}

		// Even on the last iteration it's allowed, it's the reply, rather than
		// the loop stopping, as it's been asked as many times as it can be
		last := newScripted(text(`{"result":{"city":"Berlin"}}`))
		lastLoop, lastModel := newAgentLoop(t, last)
		added, _, err := lastLoop.AgentLoop(ctx, lastModel, []llm.Message{user(t, "Where?")}, tools, nil, schema, llm.WithMaxIterations(1))
		require.NoError(t, err)
		require.Len(t, added, 1)
		assert.Equal(t, providerschema.ResultStop, added[0].Result)
		assert.JSONEq(t, `{"city":"Berlin"}`, added[0].Text())

		// Text which is JSON, but doesn't conform, is asked for again
		p := newScripted(text(`{"town":"Berlin"}`), call(toolkit.RespondName, `{"result":{"city":"Berlin"}}`))
		r, model := newAgentLoop(t, p)
		added, _, err = r.AgentLoop(ctx, model, []llm.Message{user(t, "Where?")}, tools, nil, schema)
		require.NoError(t, err)
		require.Len(t, added, 3)
		assert.Equal(t, toolkit.RespondNudge, added[1].Text())
		assert.JSONEq(t, `{"city":"Berlin"}`, added[2].Text())
	})

	t.Run("DoesntConform", func(t *testing.T) {
		// The result doesn't conform, so the model is told why
		p := newScripted(call(toolkit.RespondName, `{"result":{"town":"Berlin"}}`), call(toolkit.RespondName, `{"result":{"city":"Berlin"}}`))
		r, model := newAgentLoop(t, p)
		added, _, err := r.AgentLoop(ctx, model, []llm.Message{user(t, "Where?")}, tools, nil, schema)
		require.NoError(t, err)
		require.Len(t, added, 3)
		if results := added[1].ToolResults(); assert.Len(t, results, 1) {
			assert.True(t, results[0].IsError)
			assert.Contains(t, string(results[0].Content), "doesn't match the schema")
		}
		assert.JSONEq(t, `{"city":"Berlin"}`, added[2].Text())
		assertAnswered(t, added)
	})

	t.Run("DoesntConformLast", func(t *testing.T) {
		// The result doesn't conform when the model's been asked as many
		// times as it can be, so the call is left out of the reply, which is
		// the last message, and has no result
		p := newScripted(call(toolkit.RespondName, `{"result":{"town":"Berlin"}}`), text("Never"))
		r, model := newAgentLoop(t, p)
		added, _, err := r.AgentLoop(ctx, model, []llm.Message{user(t, "Where?")}, tools, nil, schema, llm.WithMaxIterations(1))
		require.NoError(t, err)
		require.Len(t, added, 1)
		assert.Equal(t, providerschema.ResultMaxIterations, added[0].Result)
		assert.Empty(t, added[0].ToolCalls())
		assert.Len(t, p.asked, 1)
		assertAnswered(t, added)
	})

	t.Run("RespondWithOtherTools", func(t *testing.T) {
		// The model calls the respond tool with other tools, whose results it
		// hasn't seen, so they're run, but it isn't
		both := call("echo", `{"city":"Berlin"}`)
		both.Content = append(both.Content, call(toolkit.RespondName, `{"result":{"city":"Paris"}}`).Content...)
		p := newScripted(both, call(toolkit.RespondName, `{"result":{"city":"Berlin"}}`))
		r, model := newAgentLoop(t, p)
		added, _, err := r.AgentLoop(ctx, model, []llm.Message{user(t, "Where?")}, tools, nil, schema)
		require.NoError(t, err)
		require.Len(t, added, 3)
		if results := added[1].ToolResults(); assert.Len(t, results, 2) {
			assert.False(t, results[0].IsError)
			assert.True(t, results[1].IsError)
			assert.Contains(t, string(results[1].Content), "on its own")
		}
		assert.JSONEq(t, `{"city":"Berlin"}`, added[2].Text())
		assertAnswered(t, added)
	})

	t.Run("MaxIterations", func(t *testing.T) {
		// The model never calls the respond tool, so it's asked as many times
		// as it can be, and its reply isn't a result
		p := newScripted(text("Berlin"), text("Still Berlin"), text("Never"))
		r, model := newAgentLoop(t, p)
		added, _, err := r.AgentLoop(ctx, model, []llm.Message{user(t, "Where?")}, tools, nil, schema, llm.WithMaxIterations(2))
		require.NoError(t, err)
		require.Len(t, added, 3)
		assert.Equal(t, providerschema.ResultMaxIterations, added[2].Result)
		assert.Equal(t, "Still Berlin", added[2].Text())
		assert.Len(t, p.asked, 2)
	})

	t.Run("Invalid", func(t *testing.T) {
		// A schema which isn't valid, a tool named as the respond tool, or a
		// tool choice of none, isn't, and the model isn't asked
		for name, test := range map[string]struct {
			tools []llm.Tool
			opts  []llm.Opt
		}{
			"Schema":     {tools, []llm.Opt{llm.WithJSONSchema(json.RawMessage(`{"type":1}`))}},
			"ToolName":   {[]llm.Tool{respondNamedTool{}}, []llm.Opt{schema}},
			"ToolChoice": {tools, []llm.Opt{schema, llm.WithToolChoice("none")}},
		} {
			t.Run(name, func(t *testing.T) {
				p := newScripted(text("Never"))
				r, model := newAgentLoop(t, p)
				_, _, err := r.AgentLoop(ctx, model, []llm.Message{user(t, "Where?")}, test.tools, nil, test.opts...)
				assert.ErrorIs(t, err, llm.ErrBadParameter)
				assert.Empty(t, p.asked)
			})
		}
	})
}

// respondNamedTool is a tool named as the respond tool
type respondNamedTool struct{ echoTool }

func (respondNamedTool) Name() string { return toolkit.RespondName }

func Test_AgentLoopTracer(t *testing.T) {
	// Each iteration is traced in a span of its own, in the loop's, with the
	// model's, and the tool's, in it, and the reply's result, the tools it
	// called, and the tokens it used, broken down
	ctx := context.Background()
	recorder := tracetest.NewSpanRecorder()
	tracer := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder)).Tracer("test")
	r, err := registry.New(registry.WithTracer(tracer))
	require.NoError(t, err)
	p := newScripted(call("echo", `{}`), text("Done"))
	p.usage = &llm.Usage{InputTokens: 10, OutputTokens: 5, CachedTokens: 4, ReasoningTokens: 2}
	require.NoError(t, r.Add("", p))
	model, err := r.GetModel(ctx, "model")
	require.NoError(t, err)
	_, _, err = r.AgentLoop(ctx, model, []llm.Message{user(t, "Go")}, []llm.Tool{echoTool{}}, nil)
	require.NoError(t, err)

	byName := map[string][]sdktrace.ReadOnlySpan{}
	for _, span := range recorder.Ended() {
		byName[span.Name()] = append(byName[span.Name()], span)
	}
	require.Len(t, byName["llm.registry.AgentLoop"], 1)
	loop := byName["llm.registry.AgentLoop"][0]
	iterations := byName["llm.registry.AgentLoop.Iteration"]
	require.Len(t, iterations, 2)
	for i, span := range iterations {
		assert.Equal(t, loop.SpanContext().SpanID(), span.Parent().SpanID())
		assert.Contains(t, span.Attributes(), attribute.Int("arg.iteration", i+1))
		assert.Contains(t, span.Attributes(), attribute.Int("input_tokens", 10))
		assert.Contains(t, span.Attributes(), attribute.Int("output_tokens", 5))
		assert.Contains(t, span.Attributes(), attribute.Int("cached_tokens", 4))
		assert.Contains(t, span.Attributes(), attribute.Int("reasoning_tokens", 2))
	}
	assert.Contains(t, iterations[0].Attributes(), attribute.Int("arg.messages", 1))
	assert.Contains(t, iterations[0].Attributes(), attribute.StringSlice("tool_calls", []string{"echo"}))
	assert.Contains(t, iterations[1].Attributes(), attribute.Int("arg.messages", 3))
	assert.Contains(t, iterations[1].Attributes(), attribute.String("result", "stop"))

	// The model's span is in each iteration's
	generate := byName["llm.registry.WithoutSession"]
	require.Len(t, generate, 2)
	for i, span := range generate {
		assert.Equal(t, iterations[i].SpanContext().SpanID(), span.Parent().SpanID())
	}
}

// logTool is a tool which logs as it runs, to the logger of its session
type logTool struct{ echoTool }

func (logTool) Name() string { return "log" }
func (logTool) Run(ctx context.Context, input json.RawMessage) (any, error) {
	logger := toolkit.SessionFromContext(ctx).Logger()
	logger.Debug("starting")
	logger.Info("fetching", "city", "Berlin")
	logger.WithGroup("quota").Warn("low", "remaining", 3, "note", "two words")
	logger.With(slog.Group("req", "id", 7)).Info("done")
	return input, nil
}

func Test_AgentLoopToolLog(t *testing.T) {
	ctx := context.Background()

	t.Run("Stream", func(t *testing.T) {
		// What a tool logs, at the info level or above, is streamed as lines,
		// which don't separate the replies
		p := newScripted(call("log", `{}`), text("Done"))
		p.replies[0].Content = append(p.replies[0].Content, providerschema.ContentBlock{Text: types.Ptr("Calling")})
		r, model := newAgentLoop(t, p)
		type piece struct {
			what llm.StreamType
			text string
		}
		var streamed []piece
		_, _, err := r.AgentLoop(ctx, model, []llm.Message{user(t, "Go")}, []llm.Tool{logTool{}}, func(what llm.StreamType, text string) {
			streamed = append(streamed, piece{what, text})
		})
		require.NoError(t, err)
		assert.Equal(t, []piece{
			{llm.StreamText, "Calling"},
			{llm.StreamLog, "fetching city=Berlin"},
			{llm.StreamLog, `WARN low quota.remaining=3 quota.note="two words"`},
			{llm.StreamLog, "done req.id=7"},
			{llm.StreamText, "\n\nDone"},
		}, streamed)
	})

	t.Run("NoStream", func(t *testing.T) {
		// A tool logs when the replies aren't streamed, which is discarded
		p := newScripted(call("log", `{}`), text("Done"))
		r, model := newAgentLoop(t, p)
		_, _, err := r.AgentLoop(ctx, model, []llm.Message{user(t, "Go")}, []llm.Tool{logTool{}}, nil)
		require.NoError(t, err)
	})

	t.Run("Tracer", func(t *testing.T) {
		// What a tool logs, at every level, is added to the iteration's span,
		// as events
		recorder := tracetest.NewSpanRecorder()
		tracer := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder)).Tracer("test")
		r, err := registry.New(registry.WithTracer(tracer))
		require.NoError(t, err)
		require.NoError(t, r.Add("", newScripted(call("log", `{}`), text("Done"))))
		model, err := r.GetModel(ctx, "model")
		require.NoError(t, err)
		_, _, err = r.AgentLoop(ctx, model, []llm.Message{user(t, "Go")}, []llm.Tool{logTool{}}, nil)
		require.NoError(t, err)

		var events []sdktrace.Event
		for _, span := range recorder.Ended() {
			if span.Name() == "llm.registry.AgentLoop.Iteration" {
				events = append(events, span.Events()...)
			}
		}
		require.Len(t, events, 4)
		assert.Equal(t, "starting", events[0].Name)
		assert.Contains(t, events[0].Attributes, attribute.String("level", "DEBUG"))
		assert.Equal(t, "fetching", events[1].Name)
		assert.Contains(t, events[1].Attributes, attribute.String("tool", "log"))
		assert.Contains(t, events[1].Attributes, attribute.String("city", "Berlin"))
		assert.Contains(t, events[2].Attributes, attribute.String("quota.remaining", "3"))
	})
}

// barrierTool is a tool which waits for the other tools of the barrier to
// run, so returns an error unless they run in parallel, and logs as it
// waits
type barrierTool struct {
	echoTool
	name    string
	arrived *sync.WaitGroup
}

func (t barrierTool) Name() string { return t.name }
func (t barrierTool) Run(ctx context.Context, input json.RawMessage) (any, error) {
	logger := toolkit.SessionFromContext(ctx).Logger()
	t.arrived.Done()
	waited := make(chan struct{})
	go func() { t.arrived.Wait(); close(waited) }()
	for i := range 50 {
		logger.Info("waiting", "i", i)
	}
	select {
	case <-waited:
		return t.name, nil
	case <-time.After(5 * time.Second):
		return nil, errors.New("the tools didn't run in parallel")
	}
}

func Test_AgentLoopParallel(t *testing.T) {
	// The tools the model calls run in parallel, and their results are in
	// the order of the calls, and what they log is streamed in turn
	ctx := context.Background()
	var arrived sync.WaitGroup
	arrived.Add(3)
	tools := []llm.Tool{
		barrierTool{name: "a", arrived: &arrived},
		barrierTool{name: "b", arrived: &arrived},
		barrierTool{name: "c", arrived: &arrived},
	}
	calls := &providerschema.Message{MessageMeta: providerschema.MessageMeta{Sender: "assistant"}}
	for _, name := range []string{"c", "a", "missing", "b"} {
		calls.Content = append(calls.Content, providerschema.ContentBlock{ToolCall: &providerschema.ToolCall{ID: "call-" + name, Name: name}})
	}
	p := newScripted(calls, text("Done"))
	r, model := newAgentLoop(t, p)

	// The stream isn't safe to call at once, so the race detector finds it
	// if it is
	var lines []string
	added, _, err := r.AgentLoop(ctx, model, []llm.Message{user(t, "Go")}, tools, func(what llm.StreamType, text string) {
		if what == llm.StreamLog {
			lines = append(lines, text)
		}
	})
	require.NoError(t, err)
	assert.Len(t, lines, 150)
	require.Len(t, added, 3)
	results := added[1].ToolResults()
	require.Len(t, results, 4)
	for i, name := range []string{"c", "a", "missing", "b"} {
		assert.Equal(t, "call-"+name, results[i].ID)
	}
	assert.JSONEq(t, `"c"`, string(results[0].Content))
	assert.JSONEq(t, `"a"`, string(results[1].Content))
	assert.True(t, results[2].IsError)
	assert.JSONEq(t, `"b"`, string(results[3].Content))
	assertAnswered(t, added)
}

// hangTool is a tool which doesn't return until it's released, whatever its
// context
type hangTool struct {
	echoTool
	release chan struct{}
}

func (hangTool) Name() string { return "hang" }
func (t hangTool) Run(context.Context, json.RawMessage) (any, error) {
	<-t.release
	return nil, nil
}

// bigTool is a tool whose result is 100 KB of text
type bigTool struct{ echoTool }

func (bigTool) Name() string { return "big" }
func (bigTool) Run(context.Context, json.RawMessage) (any, error) {
	return strings.Repeat("a", 100_000), nil
}

func Test_AgentLoopContext(t *testing.T) {
	// The results of tools are truncated, so the model's context, which is
	// the smaller of its input token limit and the context window option, is
	// at most two thirds full, after the request and the reply, of 10 input
	// and 5 output tokens, each result an equal share, but no fewer than 256
	// tokens, and each truncation is said in the tool's session
	ctx := context.Background()
	bigs := func(n int) *providerschema.Message {
		reply := &providerschema.Message{MessageMeta: providerschema.MessageMeta{Sender: "assistant"}}
		for i := range n {
			reply.Content = append(reply.Content, providerschema.ContentBlock{ToolCall: &providerschema.ToolCall{ID: fmt.Sprint("call-", i), Name: "big"}})
		}
		return reply
	}
	run := func(t *testing.T, p *scripted, opts ...llm.Opt) ([]llm.ToolResult, []string) {
		t.Helper()
		r, model := newAgentLoop(t, p)
		var lines []string
		added, _, err := r.AgentLoop(ctx, model, []llm.Message{user(t, "Go")}, []llm.Tool{bigTool{}}, func(what llm.StreamType, text string) {
			if what == llm.StreamLog {
				lines = append(lines, text)
			}
		}, opts...)
		require.NoError(t, err)
		require.Len(t, added, 3)
		results := added[1].ToolResults()
		for _, result := range results {
			assert.True(t, json.Valid(result.Content))
			assert.Contains(t, string(result.Content), "more characters]")
		}
		return results, lines
	}
	// tokens returns the most tokens of a result, as estimated
	tokens := func(result llm.ToolResult) int { return (len(result.Content) + 3) / 4 }

	t.Run("ContextWindow", func(t *testing.T) {
		// 4000 tokens, of which 2640 are filled, less 15 used
		results, lines := run(t, newScripted(bigs(1), text("Done")), llm.WithContextWindow(4000))
		require.Len(t, results, 1)
		assert.LessOrEqual(t, tokens(results[0]), 2625)
		assert.Greater(t, tokens(results[0]), 2000)
		if assert.Len(t, lines, 1) {
			assert.Regexp(t, `^result truncated bytes=100002 to_bytes=\d+ limit_tokens=2625$`, lines[0])
		}
	})

	t.Run("InputTokenLimit", func(t *testing.T) {
		// The model's input token limit is smaller than the option
		p := newScripted(bigs(1), text("Done"))
		p.input = types.Ptr(uint(2000))
		results, _ := run(t, p, llm.WithContextWindow(4000))
		require.Len(t, results, 1)
		assert.LessOrEqual(t, tokens(results[0]), 1305)
		assert.Greater(t, tokens(results[0]), 1000)
	})

	t.Run("Shared", func(t *testing.T) {
		// Two results share what's available
		results, lines := run(t, newScripted(bigs(2), text("Done")), llm.WithContextWindow(4000))
		require.Len(t, results, 2)
		for _, result := range results {
			assert.LessOrEqual(t, tokens(result), 1312)
			assert.Greater(t, tokens(result), 1000)
		}
		assert.Len(t, lines, 2)
	})

	t.Run("Full", func(t *testing.T) {
		// The context is already two thirds full, so a result is the fewest
		// tokens
		p := newScripted(bigs(1), text("Done"))
		p.usage = &llm.Usage{InputTokens: 3000, OutputTokens: 5}
		results, _ := run(t, p, llm.WithContextWindow(4000))
		require.Len(t, results, 1)
		assert.LessOrEqual(t, tokens(results[0]), 256)
	})

	t.Run("Tracer", func(t *testing.T) {
		// The span of the iteration the results are sent in has how many were
		// truncated, and the truncations, as events
		recorder := tracetest.NewSpanRecorder()
		tracer := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder)).Tracer("test")
		r, err := registry.New(registry.WithTracer(tracer))
		require.NoError(t, err)
		require.NoError(t, r.Add("", newScripted(bigs(2), text("Done"))))
		model, err := r.GetModel(ctx, "model")
		require.NoError(t, err)
		_, _, err = r.AgentLoop(ctx, model, []llm.Message{user(t, "Go")}, []llm.Tool{bigTool{}}, nil, llm.WithContextWindow(4000))
		require.NoError(t, err)
		var iterations []sdktrace.ReadOnlySpan
		for _, span := range recorder.Ended() {
			if span.Name() == "llm.registry.AgentLoop.Iteration" {
				iterations = append(iterations, span)
			}
		}
		require.Len(t, iterations, 2)
		assert.Contains(t, iterations[1].Attributes(), attribute.Int("truncated_results", 2))
		var events int
		for _, event := range iterations[1].Events() {
			if event.Name == "result truncated" {
				events++
			}
		}
		assert.Equal(t, 2, events)
	})
}

func Test_AgentLoopCompact(t *testing.T) {
	// The conversation is compacted, so the model's context, of 4000 tokens,
	// is at most two thirds full, without changing the caller's messages
	ctx := context.Background()
	window := llm.WithContextWindow(4000)
	big := strings.Repeat("a", 20_000)
	schemaMessage := func(role string, blocks ...providerschema.ContentBlock) *providerschema.Message {
		return &providerschema.Message{MessageMeta: providerschema.MessageMeta{Sender: role, Content: blocks}}
	}
	textBlock := func(text string) providerschema.ContentBlock {
		return providerschema.ContentBlock{Text: types.Ptr(text)}
	}

	t.Run("RemoveResults", func(t *testing.T) {
		// The results of tools of earlier turns are removed, with a note
		old := providerschema.NewToolResult("old", "echo", big)
		messages := []llm.Message{
			schemaMessage("user", textBlock("First")),
			schemaMessage("assistant", providerschema.ContentBlock{ToolCall: &providerschema.ToolCall{ID: "old", Name: "echo"}}),
			schemaMessage("tool", old),
			schemaMessage("user", textBlock("Second")),
		}
		p := newScripted(text("Done"))
		r, model := newAgentLoop(t, p)
		added, _, err := r.AgentLoop(ctx, model, messages, tools(), nil, window)
		require.NoError(t, err)
		require.Len(t, added, 1)
		assert.Equal(t, providerschema.ResultStop, added[0].Result)
		require.Len(t, p.asked, 1)
		require.Len(t, p.asked[0].messages, 4)
		assert.Contains(t, string(p.asked[0].messages[2].ToolResults()[0].Content), "was removed")

		// The caller's message isn't changed
		assert.Len(t, messages[2].ToolResults()[0].Content, len(old.ToolResult.Content))
	})

	t.Run("DropTurns", func(t *testing.T) {
		// Earlier turns are dropped, oldest first, with the results of the
		// tools a reply called, so each call has its result, and the latest
		// user message is kept
		messages := []llm.Message{
			schemaMessage("system", textBlock("Be brief")),
			schemaMessage("user", textBlock("First")),
			schemaMessage("assistant", providerschema.ContentBlock{ToolCall: &providerschema.ToolCall{ID: "old", Name: "echo", Input: json.RawMessage(`"` + big + `"`)}}),
			schemaMessage("tool", providerschema.NewToolResult("old", "echo", "ok")),
			schemaMessage("user", textBlock("Second")),
		}
		p := newScripted(text("Done"))
		r, model := newAgentLoop(t, p)
		added, _, err := r.AgentLoop(ctx, model, messages, tools(), nil, window)
		require.NoError(t, err)
		require.Len(t, added, 1)
		require.Len(t, p.asked, 1)
		var roles []string
		for _, message := range p.asked[0].messages {
			roles = append(roles, message.Role())
		}
		assert.Equal(t, []string{"system", "user"}, roles)
		assert.Equal(t, "Second", p.asked[0].messages[1].Text())
		assert.Len(t, messages, 5)
	})

	t.Run("MaxContext", func(t *testing.T) {
		// The latest user message doesn't fit in the model's whole context,
		// so the model isn't asked, and the reply is empty, with max_context
		// as its result
		p := newScripted(text("Never"))
		r, model := newAgentLoop(t, p)
		added, usage, err := r.AgentLoop(ctx, model, []llm.Message{schemaMessage("user", textBlock(big))}, tools(), nil, window)
		require.NoError(t, err)
		require.Len(t, added, 1)
		assert.Equal(t, "assistant", added[0].Role())
		assert.Equal(t, providerschema.ResultMaxContext, added[0].Result)
		assert.Empty(t, added[0].Text())
		assert.Empty(t, p.asked)
		assert.Equal(t, llm.Usage{}, withoutDuration(usage))
	})

	t.Run("Attachment", func(t *testing.T) {
		// An attachment is an estimate of tokens, rather than its data as
		// text, so a large image fits, and the model is asked
		image := providerschema.ContentBlock{Resource: &providerschema.Resource{ResourceMeta: providerschema.ResourceMeta{
			ContentType: "image/png",
			Data:        make([]byte, 1<<20),
		}}}
		p := newScripted(text("A cat"))
		r, model := newAgentLoop(t, p)
		added, _, err := r.AgentLoop(ctx, model, []llm.Message{schemaMessage("user", textBlock("What's this?"), image)}, tools(), nil, window)
		require.NoError(t, err)
		require.Len(t, added, 1)
		assert.Equal(t, providerschema.ResultStop, added[0].Result)
		require.Len(t, p.asked, 1)
		assert.Len(t, p.asked[0].messages[0].Resources(), 1)
	})

	t.Run("MessageMeta", func(t *testing.T) {
		// Messages which are the meta of messages, by value or pointer, are
		// compacted too: an earlier turn's result is removed, and the latest
		// turn's is truncated
		messages := []llm.Message{
			&providerschema.MessageMeta{Sender: "user", Content: []providerschema.ContentBlock{textBlock("First")}},
			providerschema.MessageMeta{Sender: "assistant", Content: []providerschema.ContentBlock{{ToolCall: &providerschema.ToolCall{ID: "old", Name: "echo"}}}},
			providerschema.MessageMeta{Sender: "tool", Content: []providerschema.ContentBlock{providerschema.NewToolResult("old", "echo", big)}},
			&providerschema.MessageMeta{Sender: "user", Content: []providerschema.ContentBlock{textBlock("Second")}},
			providerschema.MessageMeta{Sender: "assistant", Content: []providerschema.ContentBlock{{ToolCall: &providerschema.ToolCall{ID: "new", Name: "echo"}}}},
			&providerschema.MessageMeta{Sender: "tool", Content: []providerschema.ContentBlock{providerschema.NewToolResult("new", "echo", strings.Repeat("b", 100_000))}},
		}
		p := newScripted(text("Done"))
		r, model := newAgentLoop(t, p)
		_, _, err := r.AgentLoop(ctx, model, messages, tools(), nil, window)
		require.NoError(t, err)
		require.Len(t, p.asked, 1)
		sent := p.asked[0].messages
		require.Len(t, sent, 6)
		assert.Contains(t, string(sent[2].ToolResults()[0].Content), "was removed")
		assert.Contains(t, string(sent[5].ToolResults()[0].Content), "more characters]")
		assert.Less(t, len(sent[5].ToolResults()[0].Content), 4*2640)
	})

	t.Run("Loop", func(t *testing.T) {
		// The model calls a tool whose result is large, twice, so the first
		// result is truncated when it's sent, and removed when the second is,
		// which is truncated, but the first is returned as it was sent
		p := newScripted(call("big", `{}`), call("big", `{}`), text("Done"))
		r, model := newAgentLoop(t, p)
		added, _, err := r.AgentLoop(ctx, model, []llm.Message{user(t, "Go")}, []llm.Tool{bigTool{}}, nil, window)
		require.NoError(t, err)
		require.Len(t, added, 5)
		require.Len(t, p.asked, 3)

		// The first result is truncated, when it's sent
		first := p.asked[1].messages[2].ToolResults()[0]
		assert.Contains(t, string(first.Content), "more characters]")

		// It's removed when the second is sent, which is truncated
		third := p.asked[2].messages
		require.Len(t, third, 5)
		assert.Contains(t, string(third[2].ToolResults()[0].Content), "was removed")
		assert.Contains(t, string(third[4].ToolResults()[0].Content), "more characters]")

		// The results are returned as they were first sent
		assert.Equal(t, string(first.Content), string(added[1].ToolResults()[0].Content))
		assert.Equal(t, string(third[4].ToolResults()[0].Content), string(added[3].ToolResults()[0].Content))
	})
}

// tools returns the tools of the tests
func tools() []llm.Tool { return []llm.Tool{echoTool{}} }

// chattyTool is a tool which ignores its context, and logs until it's
// released
type chattyTool struct {
	echoTool
	release, stopped chan struct{}
}

func (chattyTool) Name() string { return "chatty" }
func (t chattyTool) Run(ctx context.Context, _ json.RawMessage) (any, error) {
	defer close(t.stopped)
	logger := toolkit.SessionFromContext(ctx).Logger()
	for {
		select {
		case <-t.release:
			return nil, nil
		default:
			logger.Info("still here")
			time.Sleep(100 * time.Microsecond)
		}
	}
}

func Test_AgentLoopTimedOutLog(t *testing.T) {
	// A tool which times out, and keeps logging as the next reply streams,
	// doesn't call the stream at once with it, which the race detector
	// finds, as the stream isn't safe to call at once
	ctx := context.Background()
	tool := chattyTool{release: make(chan struct{}), stopped: make(chan struct{})}
	p := newScripted(call("chatty", `{}`), text("Done"))
	p.delay = 20 * time.Millisecond
	r, model := newAgentLoop(t, p)
	var streamed []string
	added, _, err := r.AgentLoop(ctx, model, []llm.Message{user(t, "Go")}, []llm.Tool{tool}, func(_ llm.StreamType, text string) {
		streamed = append(streamed, text)
	}, llm.WithToolTimeout(20*time.Millisecond))
	require.NoError(t, err)
	require.Len(t, added, 3)
	assert.Contains(t, string(added[1].ToolResults()[0].Content), "timed out")
	assert.Contains(t, streamed, "Done")

	// Nothing is streamed once the loop returns, though the tool still logs
	n := len(streamed)
	time.Sleep(20 * time.Millisecond)
	assert.Len(t, streamed, n)
	close(tool.release)
	<-tool.stopped
}

func Test_AgentLoopNilMessage(t *testing.T) {
	// A message which is nil, or a nil pointer, isn't valid, so the model
	// isn't asked
	ctx := context.Background()
	for name, message := range map[string]llm.Message{
		"Nil":        nil,
		"NilPointer": (*providerschema.Message)(nil),
		"NilMeta":    (*providerschema.MessageMeta)(nil),
	} {
		t.Run(name, func(t *testing.T) {
			p := newScripted(text("Never"))
			r, model := newAgentLoop(t, p)
			_, _, err := r.AgentLoop(ctx, model, []llm.Message{user(t, "Hi"), message}, tools(), nil)
			assert.ErrorIs(t, err, llm.ErrBadParameter)
			assert.Empty(t, p.asked)
		})
	}
}

func Test_AgentLoopSystemPrompts(t *testing.T) {
	// The system prompt is the system_prompt option, followed by the system
	// prompts of the tools which are the registry's toolkit's, once each, and
	// the instruction to call the respond tool, when there's a schema
	ctx := context.Background()
	tk, err := toolkit.New(ctx)
	require.NoError(t, err)
	require.NoError(t, tk.Add(toolkit.NewNamespace("weather", "Use metric units.", namedEcho{name: "current"}, namedEcho{name: "forecast"})))
	require.NoError(t, tk.Add(toolkit.NewNamespace("news", "Cite the source.", namedEcho{name: "headlines"})))
	run := func(t *testing.T, p *scripted, tools []llm.Tool, opts ...llm.Opt) string {
		t.Helper()
		r, err := registry.New(registry.WithToolkit(tk))
		require.NoError(t, err)
		require.NoError(t, r.Add("", p))
		model, err := r.GetModel(ctx, "model")
		require.NoError(t, err)
		_, _, err = r.AgentLoop(ctx, model, []llm.Message{user(t, "Hi")}, tools, nil, opts...)
		require.NoError(t, err)
		require.NotEmpty(t, p.asked)
		system, _ := llm.GetOpt[string](p.asked[0].opts, llm.SystemPromptKey)
		return system
	}
	toolsOf := func(t *testing.T, names ...string) []llm.Tool {
		t.Helper()
		tools, err := tk.Tools(ctx, names...)
		require.NoError(t, err)
		return tools
	}

	t.Run("Tools", func(t *testing.T) {
		system := run(t, newScripted(text("Hello")), toolsOf(t, "weather:*", "news:headlines"), llm.WithSystemPrompt("Be brief"))
		assert.Equal(t, "Be brief\n\nUse metric units.\n\nCite the source.", system)
	})

	t.Run("NoSystem", func(t *testing.T) {
		system := run(t, newScripted(text("Hello")), toolsOf(t, "news:headlines"))
		assert.Equal(t, "Cite the source.", system)
	})

	t.Run("OtherTools", func(t *testing.T) {
		// A tool which isn't the toolkit's has no system prompt
		system := run(t, newScripted(text("Hello")), []llm.Tool{echoTool{}}, llm.WithSystemPrompt("Be brief"))
		assert.Equal(t, "Be brief", system)
	})

	t.Run("Schema", func(t *testing.T) {
		system := run(t, newScripted(call(toolkit.RespondName, `{"result":"ok"}`)), toolsOf(t, "news:*"),
			llm.WithSystemPrompt("Be brief"), llm.WithJSONSchema(json.RawMessage(`{"type":"string"}`)))
		assert.Equal(t, "Be brief\n\nCite the source.\n\n"+toolkit.RespondPrompt, system)
	})
}

// countingNamespace is a namespace whose system prompt changes each time
// it's read, as one which has the time can, and counts how many times
type countingNamespace struct {
	tools []llm.Tool
	reads *atomic.Int32
}

func (n countingNamespace) Name() string      { return "clock" }
func (n countingNamespace) Tools() []llm.Tool { return n.tools }
func (n countingNamespace) SystemPrompt() string {
	return fmt.Sprint("Read ", n.reads.Add(1), " times.")
}

// Notify does nothing, as the namespace's tools don't change
func (countingNamespace) Description() string { return "" }

func (countingNamespace) Notify(func(eventschema.EventAction, string)) {}

func Test_AgentLoopToolPromptsOnce(t *testing.T) {
	// The system prompt of the namespace of the tools is read once, however
	// many of its tools there are, and isn't added when the tool choice is
	// none, as the model has no tools
	ctx := context.Background()
	run := func(t *testing.T, opts ...llm.Opt) (string, int32) {
		t.Helper()
		var reads atomic.Int32
		tk, err := toolkit.New(ctx)
		require.NoError(t, err)
		require.NoError(t, tk.Add(countingNamespace{[]llm.Tool{namedEcho{name: "now"}, namedEcho{name: "alarm"}}, &reads}))
		tools, err := tk.Tools(ctx, "clock:*")
		require.NoError(t, err)
		r, err := registry.New(registry.WithToolkit(tk))
		require.NoError(t, err)
		p := newScripted(text("Hello"))
		require.NoError(t, r.Add("", p))
		model, err := r.GetModel(ctx, "model")
		require.NoError(t, err)
		_, _, err = r.AgentLoop(ctx, model, []llm.Message{user(t, "Hi")}, tools, nil, opts...)
		require.NoError(t, err)
		system, _ := llm.GetOpt[string](p.asked[0].opts, llm.SystemPromptKey)
		return system, reads.Load()
	}

	t.Run("Once", func(t *testing.T) {
		system, reads := run(t, llm.WithSystemPrompt("Be brief"))
		assert.Equal(t, "Be brief\n\nRead 1 times.", system)
		assert.Equal(t, int32(1), reads)
	})

	t.Run("ToolChoiceNone", func(t *testing.T) {
		system, _ := run(t, llm.WithSystemPrompt("Be brief"), llm.WithToolChoice("none"))
		assert.Equal(t, "Be brief", system)
	})
}
