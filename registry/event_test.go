package registry_test

import (
	"context"
	"encoding/json"
	"testing"
	"uuid"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	agents "github.com/mutablelogic/go-llm/agents"
	providerschema "github.com/mutablelogic/go-llm/provider/schema"
	registry "github.com/mutablelogic/go-llm/registry"
	schema "github.com/mutablelogic/go-llm/registry/schema"
	test "github.com/mutablelogic/go-llm/registry/test"
	toolkit "github.com/mutablelogic/go-llm/toolkit"
	jsonschema "github.com/mutablelogic/go-server/pkg/jsonschema"
	types "github.com/mutablelogic/go-server/pkg/types"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

func Test_RunEvent(t *testing.T) {
	ctx := context.Background()
	input := json.RawMessage(`{"title":"Call","prompt":"Remind David to call his mother","context":"Now: 18:00"}`)
	reply := func() *providerschema.Message {
		return call(toolkit.RespondName, `{"result":{"message":"Call your mother","importance":"alert"}}`)
	}

	// A registry with the agents the server has, such as the event agent,
	// and a model which replies as the event agent does
	tk := test.NewToolkit(t)
	require.NoError(t, tk.Add(toolkit.NewNamespace("heartbeat", "", eventTool{})))
	r, err := registry.New(registry.WithToolkit(tk), registry.WithAgentStore(test.NewAgentStore()), registry.WithSessionStore(test.NewSessionStore()))
	require.NoError(t, err)
	p := newScripted(reply(), reply(), reply())
	p.models = []string{"model", "other"}
	require.NoError(t, r.Add("", p))
	all, err := agents.All()
	require.NoError(t, err)
	for _, meta := range all {
		_, _, err := r.CreateAgent(ctx, meta)
		require.NoError(t, err)
	}
	parent, err := r.CreateSession(ctx, schema.SessionMeta{GeneratorMeta: schema.GeneratorMeta{Model: types.Ptr("model"), Tools: []string{"*"}}})
	require.NoError(t, err)

	// The event's session is created, with its identifier, and the model of
	// the session it's for, and the event agent is run with it, and replies
	// with the message, and its importance
	event := uuid.New()
	response, err := r.RunEvent(ctx, event, schema.RunEventRequest{Session: parent.ID, Input: input, Without: []string{"heartbeat"}}, nil)
	require.NoError(t, err)
	assert.Equal(t, event, response.Session.ID)
	assert.Equal(t, "scripted:model", types.Value(response.Session.Model))

	// With the tools of the session it's for, apart from the heartbeats'
	require.NotEmpty(t, p.asked)
	assert.Contains(t, p.asked[0].tools, "weather__current")
	assert.Contains(t, p.asked[0].tools, "weather__forecast")
	for _, tool := range p.asked[0].tools {
		assert.NotContains(t, tool, "heartbeat", "an event doesn't schedule events")
	}
	assert.Equal(t, registry.DefaultEventAgent, response.Run.Name)
	assert.JSONEq(t, `{"message":"Call your mother","importance":"alert"}`, string(response.Run.Reply))
	require.NotEmpty(t, p.asked)
	assert.Equal(t, "<system-reminder>\nNow: 18:00\n</system-reminder>\nThe event \"Call\" has fired. Its instruction is:\n\nRemind David to call his mother\n", p.asked[0].messages[0].Text())

	// What it did, which is its prompt, and its reply, is in the session, with
	// its usage
	messages, err := r.ListSessionMessages(ctx, event, schema.MessageListRequest{})
	require.NoError(t, err)
	runMessages := len(response.Run.Messages)
	require.NotZero(t, runMessages)
	assert.Equal(t, uint(runMessages), messages.Count)
	assert.Equal(t, p.asked[0].messages[0].Text(), messages.Body[0].Text())
	assert.NotZero(t, response.Session.Usage.InputTokens)

	// The event agent is internal, so it isn't a tool which models can call,
	// as other agents are
	_, err = tk.ToolNames(ctx, "agent:event")
	assert.ErrorIs(t, err, llm.ErrNotFound)
	_, err = tk.ToolNames(ctx, "agent:translate")
	assert.NoError(t, err)

	// And then it's the same session, which has each run
	response, err = r.RunEvent(ctx, event, schema.RunEventRequest{Session: parent.ID, Input: input}, nil)
	require.NoError(t, err)
	assert.Equal(t, event, response.Session.ID)
	messages, err = r.ListSessionMessages(ctx, event, schema.MessageListRequest{})
	require.NoError(t, err)
	assert.Equal(t, uint(runMessages+len(response.Run.Messages)), messages.Count)

	// Which takes the model, when it's changed
	_, err = r.UpdateSession(ctx, parent.ID, schema.SessionMeta{GeneratorMeta: schema.GeneratorMeta{Model: types.Ptr("other")}})
	require.NoError(t, err)
	response, err = r.RunEvent(ctx, event, schema.RunEventRequest{Session: parent.ID, Input: input}, nil)
	require.NoError(t, err)
	assert.Equal(t, "scripted:other", types.Value(response.Session.Model))

	// Errors: no event, or session, the same, a session which doesn't exist,
	// or has no model, an agent which doesn't exist, and an input which isn't
	// the agent's
	for name, test := range map[string]struct {
		id  uuid.UUID
		req schema.RunEventRequest
		err error
	}{
		"NoEvent":   {uuid.Nil(), schema.RunEventRequest{Session: parent.ID, Input: input}, llm.ErrBadParameter},
		"NoSession": {event, schema.RunEventRequest{Input: input}, llm.ErrBadParameter},
		"Same":      {parent.ID, schema.RunEventRequest{Session: parent.ID, Input: input}, llm.ErrBadParameter},
		"Missing":   {event, schema.RunEventRequest{Session: uuid.New(), Input: input}, llm.ErrNotFound},
		"NoAgent":   {event, schema.RunEventRequest{Session: parent.ID, Agent: "missing", Input: input}, llm.ErrNotFound},
		"Input":     {event, schema.RunEventRequest{Session: parent.ID, Input: json.RawMessage(`{"title":"Call","prompt":"Remind David"}`)}, llm.ErrBadParameter},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := r.RunEvent(ctx, test.id, test.req, nil)
			assert.ErrorIs(t, err, test.err)
		})
	}
	noModel, err := r.CreateSession(ctx, schema.SessionMeta{})
	require.NoError(t, err)
	_, err = r.RunEvent(ctx, uuid.New(), schema.RunEventRequest{Session: noModel.ID, Input: input}, nil)
	assert.ErrorIs(t, err, llm.ErrBadParameter)

	// Without a session store
	noStore, err := registry.New()
	require.NoError(t, err)
	_, err = noStore.RunEvent(ctx, event, schema.RunEventRequest{Session: parent.ID, Input: input}, nil)
	assert.ErrorIs(t, err, llm.ErrServiceUnavailable)
}

// eventTool is a tool of the heartbeat namespace, which an event isn't given
type eventTool struct{ toolkit.Base }

func (eventTool) Name() string                    { return "heartbeats" }
func (eventTool) Description() string             { return "Schedules heartbeats" }
func (eventTool) InputSchema() *jsonschema.Schema { return nil }
func (eventTool) Run(context.Context, json.RawMessage) (any, error) {
	return nil, nil
}
