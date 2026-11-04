package registry_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	providerschema "github.com/mutablelogic/go-llm/provider/schema"
	schema "github.com/mutablelogic/go-llm/registry/schema"
	toolkit "github.com/mutablelogic/go-llm/toolkit"
	types "github.com/mutablelogic/go-server/pkg/types"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

// says returns a message of the role with the text, and the tokens
func says(role, text string, tokens uint64) providerschema.Message {
	return providerschema.Message{MessageMeta: providerschema.MessageMeta{Sender: role, Content: []providerschema.ContentBlock{{Text: types.Ptr(text)}}}, Tokens: tokens}
}

func Test_WithSession(t *testing.T) {
	ctx := context.Background()
	meta := schema.GeneratorMeta{Model: "model", System: "Be brief"}
	hi := []llm.Message{says("user", "Hi", 0)}

	t.Run("Reply", func(t *testing.T) {
		// The messages to add are the message, measured, and the reply, whose
		// tokens are its output, and the conversation's tokens are of them,
		// with the overhead, of the request's input less its messages
		p := newScripted(text("Hello"))
		r := newAskRegistry(t, p)
		response, err := r.WithSession(ctx, newSession(meta), hi, nil)
		require.NoError(t, err)
		require.Len(t, response.Messages, 2)
		assert.Equal(t, "Hi", response.Messages[0].Text())
		assert.Equal(t, uint64(4), response.Messages[0].Tokens)
		assert.Equal(t, "Hello", response.Messages[1].Text())
		assert.Equal(t, uint64(5), response.Messages[1].Tokens)
		assert.Equal(t, providerschema.ResultStop, response.Messages[1].Result)
		assert.Equal(t, uint(4), response.Input)
		assert.Equal(t, uint(5), response.Output)
		assert.Equal(t, uint(6), response.Overhead)
		assert.Equal(t, llm.Usage{InputTokens: 10, OutputTokens: 5}, response.Usage)

		// The model is asked with the system prompt
		require.Len(t, p.asked, 1)
		system, _ := llm.GetOpt[string](p.asked[0].opts, llm.SystemPromptKey)
		assert.Equal(t, "Be brief", system)
	})

	t.Run("Earlier", func(t *testing.T) {
		// The earlier messages are sent before the messages, and counted by
		// their own tokens, rather than measured, and aren't changed or added
		earlier := []providerschema.Message{says("user", "First", 100), says("assistant", "Reply", 50)}
		p := newScripted(text("Hello"))
		r := newAskRegistry(t, p)
		response, err := r.WithSession(ctx, newSession(meta, earlier...), hi, nil)
		require.NoError(t, err)
		require.Len(t, response.Messages, 2)
		assert.Equal(t, uint(104), response.Input)
		assert.Equal(t, uint(55), response.Output)
		require.Len(t, p.asked, 1)
		require.Len(t, p.asked[0].messages, 3)
		assert.Equal(t, "First", p.asked[0].messages[0].Text())
		assert.Equal(t, uint64(100), earlier[0].Tokens)
	})

	t.Run("Tools", func(t *testing.T) {
		// The tools the model calls are run, and their results added, and
		// measured
		p := newScripted(call("test_echo", `{"city":"Berlin"}`), text("Sunny"))
		r := newAskRegistry(t, p)
		withTools := meta
		withTools.Tools = []string{"test:echo"}
		response, err := r.WithSession(ctx, newSession(withTools), hi, nil)
		require.NoError(t, err)
		require.Len(t, response.Messages, 4)
		assert.Len(t, response.Messages[1].ToolCalls(), 1)
		assert.Equal(t, "tool", response.Messages[2].Role())
		if results := response.Messages[2].ToolResults(); assert.Len(t, results, 1) {
			assert.JSONEq(t, `{"city":"Berlin"}`, string(results[0].Content))
		}
		assert.NotZero(t, response.Messages[2].Tokens)
		assert.Equal(t, "Sunny", response.Messages[3].Text())
		assert.Equal(t, uint(4)+uint(response.Messages[2].Tokens), response.Input)
		assert.Equal(t, uint(10), response.Output)
		assert.Equal(t, llm.Usage{InputTokens: 20, OutputTokens: 10}, response.Usage)
	})

	t.Run("Schema", func(t *testing.T) {
		// With a schema, the reply is the result, as JSON, when its result is
		// stop
		p := newScripted(call(toolkit.RespondName, `{"result":{"city":"Berlin"}}`))
		r := newAskRegistry(t, p)
		withSchema := meta
		withSchema.Schema = json.RawMessage(`{"type":"object","properties":{"city":{"type":"string"}},"required":["city"]}`)
		response, err := r.WithSession(ctx, newSession(withSchema), hi, nil)
		require.NoError(t, err)
		reply := response.Messages[len(response.Messages)-1]
		assert.Equal(t, providerschema.ResultStop, reply.Result)
		assert.JSONEq(t, `{"city":"Berlin"}`, reply.Text())
	})

	t.Run("MaxIterations", func(t *testing.T) {
		// The model is asked as many times as DefaultMaxIterations, rather
		// than as Ask is, unless the option says otherwise
		var replies []*providerschema.Message
		for range 20 {
			replies = append(replies, call("test_echo", `{}`))
		}
		p := newScripted(replies...)
		r := newAskRegistry(t, p)
		withTools := meta
		withTools.Tools = []string{"test:echo"}
		_, err := r.WithSession(ctx, newSession(withTools), hi, nil)
		require.NoError(t, err)
		assert.Len(t, p.asked, 10)

		p = newScripted(replies...)
		r = newAskRegistry(t, p)
		withTools.Opt = map[string]json.RawMessage{llm.MaxIterationsKey: json.RawMessage(`2`)}
		response, err := r.WithSession(ctx, newSession(withTools), hi, nil)
		require.NoError(t, err)
		assert.Len(t, p.asked, 2)
		assert.Equal(t, providerschema.ResultMaxIterations, response.Messages[len(response.Messages)-1].Result)
	})

	t.Run("AnyMessage", func(t *testing.T) {
		// A message which isn't of the provider schema is kept as one, with
		// its text, attachments, whose data is read, and tool results
		p := newScripted(text("A cat"))
		r := newAskRegistry(t, p)
		message := plainMessage{
			text:      "What's this?",
			resources: []llm.Resource{resource{name: "cat.png", typ: "image/png", data: []byte("png")}},
			results:   []llm.ToolResult{{ID: "1", Name: "echo", Content: json.RawMessage(`"ok"`)}},
		}
		response, err := r.WithSession(ctx, newSession(meta), []llm.Message{message}, nil)
		require.NoError(t, err)
		require.Len(t, response.Messages, 2)
		kept := response.Messages[0]
		assert.Equal(t, "user", kept.Role())
		assert.Equal(t, "What's this?", kept.Text())
		require.Len(t, kept.Content, 3)
		require.NotNil(t, kept.Content[1].Resource)
		assert.Equal(t, "image/png", kept.Content[1].Resource.ContentType)
		assert.Equal(t, []byte("png"), kept.Content[1].Resource.Data)
		require.NotNil(t, kept.Content[2].ToolResult)
		assert.JSONEq(t, `"ok"`, string(kept.Content[2].ToolResult.Content))
		assert.NotZero(t, kept.Tokens)

		// The model is sent the message as it's kept
		require.Len(t, p.asked, 1)
		assert.Len(t, p.asked[0].messages[0].Resources(), 1)

		// A message whose attachment can't be read isn't valid
		message.resources = []llm.Resource{resource{name: "cat.png", typ: "image/png", err: errors.New("can't read")}}
		_, err = r.WithSession(ctx, newSession(meta), []llm.Message{message}, nil)
		assert.ErrorIs(t, err, llm.ErrBadParameter)
		assert.ErrorContains(t, err, "can't read")
	})

	t.Run("Errors", func(t *testing.T) {
		r := newAskRegistry(t, newScripted(text("Never")))
		_, err := r.WithSession(ctx, newSession(schema.GeneratorMeta{}), hi, nil)
		assert.ErrorIs(t, err, llm.ErrBadParameter)
		_, err = r.WithSession(ctx, newSession(meta), nil, nil)
		assert.ErrorIs(t, err, llm.ErrBadParameter)
		_, err = r.WithSession(ctx, newSession(schema.GeneratorMeta{Model: "model", Schema: json.RawMessage(`{"type":1}`)}), hi, nil)
		assert.ErrorIs(t, err, llm.ErrBadParameter)
		_, err = r.WithSession(ctx, newSession(schema.GeneratorMeta{Model: "missing"}), hi, nil)
		assert.ErrorIs(t, err, llm.ErrNotFound)
		_, err = r.WithSession(ctx, newSession(meta), []llm.Message{nil}, nil)
		assert.ErrorIs(t, err, llm.ErrBadParameter)
	})
}

// plainMessage is a message from the user, which isn't of the provider
// schema
type plainMessage struct {
	text      string
	resources []llm.Resource
	results   []llm.ToolResult
}

func (m plainMessage) Role() string                  { return "user" }
func (m plainMessage) Text() string                  { return m.text }
func (m plainMessage) Resources() []llm.Resource     { return m.resources }
func (m plainMessage) ToolCalls() []llm.ToolCall     { return nil }
func (m plainMessage) ToolResults() []llm.ToolResult { return m.results }

// resource is an attachment, whose data is read, or fails with the error
type resource struct {
	name, typ string
	data      []byte
	err       error
}

func (r resource) URI() string         { return "file:///" + r.name }
func (r resource) Name() string        { return r.name }
func (r resource) Description() string { return "" }
func (r resource) Type() string        { return r.typ }
func (r resource) Read(context.Context) (llm.ReadFunc, error) {
	if r.err != nil {
		return nil, r.err
	}
	return llm.ReadBytes(r.data), nil
}

// newSession returns a session, asked as the meta says, with the messages
func newSession(meta schema.GeneratorMeta, messages ...providerschema.Message) schema.Session {
	return schema.Session{SessionMeta: schema.SessionMeta{GeneratorMeta: meta}, Messages: messages}
}
