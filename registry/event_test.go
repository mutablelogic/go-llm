package registry_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
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
	assert.True(t, response.Session.Event, "it's an event's session")
	assert.False(t, parent.Event)

	// Which isn't chatted in, and stays an event's when it's changed, and a
	// session created by the API isn't an event's, whatever it says
	_, err = r.Chat(ctx, event, []llm.Message{user(t, "Hello")}, nil)
	assert.ErrorIs(t, err, llm.ErrForbidden)
	updated, err := r.UpdateSession(ctx, event, schema.SessionMeta{GeneratorMeta: schema.GeneratorMeta{System: types.Ptr("Be brief")}, Event: false})
	require.NoError(t, err)
	assert.True(t, updated.Event)
	created, err := r.CreateSession(ctx, schema.SessionMeta{GeneratorMeta: schema.GeneratorMeta{Model: types.Ptr("model")}, Event: true})
	require.NoError(t, err)
	assert.False(t, created.Event)

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

	// An agent which the event's agent calls, whose own tools are the
	// heartbeats', isn't given them either
	_, _, err = r.CreateAgent(ctx, schema.AgentMeta{
		PromptMeta:    schema.PromptMeta{Name: "helper", Prompt: types.Ptr("Help")},
		GeneratorMeta: schema.GeneratorMeta{Tools: []string{"heartbeat:*", "weather:current"}},
	})
	require.NoError(t, err)
	p.replies = append(p.replies, call("agent__helper", `{}`), text("Helped"), reply())
	asked := len(p.asked)
	_, err = r.RunEvent(ctx, event, schema.RunEventRequest{Session: parent.ID, Input: input, Without: []string{"heartbeat"}}, nil)
	require.NoError(t, err)
	var helped bool
	for _, a := range p.asked[asked:] {
		if len(a.messages) > 0 && a.messages[0].Text() == "Help" {
			helped = true
			assert.Equal(t, []string{"weather__current"}, a.tools, "the helper isn't given the heartbeats' tools")
		}
	}
	assert.True(t, helped, "the helper was run")

	// An agent made internal, with nothing else changed, isn't a tool, and,
	// made not internal, is one again
	_, err = r.UpdateAgent(ctx, "helper", schema.AgentMeta{PromptMeta: schema.PromptMeta{Internal: types.Ptr(true)}})
	require.NoError(t, err)
	_, err = tk.ToolNames(ctx, "agent:helper")
	assert.ErrorIs(t, err, llm.ErrNotFound)
	_, err = r.UpdateAgent(ctx, "helper", schema.AgentMeta{PromptMeta: schema.PromptMeta{Internal: types.Ptr(false)}})
	require.NoError(t, err)
	_, err = tk.ToolNames(ctx, "agent:helper")
	assert.NoError(t, err)

	// A tool which a model had before the agent was made internal isn't run
	held, err := tk.Tools(ctx, "agent:helper")
	require.NoError(t, err)
	require.Len(t, held, 1)
	_, err = r.UpdateAgent(ctx, "helper", schema.AgentMeta{PromptMeta: schema.PromptMeta{Internal: types.Ptr(true)}})
	require.NoError(t, err)
	_, err = held[0].Run(ctx, json.RawMessage(`{}`))
	assert.ErrorIs(t, err, llm.ErrNotFound)
	_, err = r.UpdateAgent(ctx, "helper", schema.AgentMeta{PromptMeta: schema.PromptMeta{Internal: types.Ptr(false)}})
	require.NoError(t, err)

	// A run isn't stored, when the session's messages are forgotten as it
	// runs, as a chat's reply isn't
	p.delay = 300 * time.Millisecond
	p.replies = append(p.replies, reply())
	done := make(chan error)
	go func() {
		_, err := r.RunEvent(ctx, event, schema.RunEventRequest{Session: parent.ID, Input: input}, func(llm.StreamType, string) {})
		done <- err
	}()
	time.Sleep(100 * time.Millisecond)
	_, err = r.ForgetSessionMessages(ctx, event)
	require.NoError(t, err)
	assert.ErrorIs(t, <-done, llm.ErrConflict)
	p.delay = 0
	messages, err = r.ListSessionMessages(ctx, event, schema.MessageListRequest{})
	require.NoError(t, err)
	assert.Zero(t, messages.Count, "the forgotten messages stay forgotten")

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

func Test_Ready(t *testing.T) {
	// The registry is ready once it's run, and has synced its providers
	r, err := registry.New()
	require.NoError(t, err)
	select {
	case <-r.Ready():
		t.Fatal("ready before it's run")
	default:
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error)
	go func() { done <- r.Run(ctx, nil) }()
	select {
	case <-r.Ready():
	case <-time.After(5 * time.Second):
		t.Fatal("not ready once it's run")
	}
	cancel()
	assert.NoError(t, <-done)
}

func Test_EventOutcomes(t *testing.T) {
	ctx := context.Background()
	input := json.RawMessage(`{"title":"Call","prompt":"Remind David to call his mother","context":"Now: 18:00"}`)
	respond := func(message, importance string) *providerschema.Message {
		return call(toolkit.RespondName, `{"result":{"message":"`+message+`","importance":"`+importance+`"}}`)
	}

	// A registry with an event store, and the agents the server has
	events := test.NewEventStore()
	r, err := registry.New(registry.WithToolkit(test.NewToolkit(t)), registry.WithAgentStore(test.NewAgentStore()), registry.WithSessionStore(test.NewSessionStore()), registry.WithEventStore(events))
	require.NoError(t, err)
	p := newScripted(respond("Call your mother", "alert"), respond("Nothing new", "ignore"))
	require.NoError(t, r.Add("", p))
	all, err := agents.All()
	require.NoError(t, err)
	for _, meta := range all {
		_, _, err := r.CreateAgent(ctx, meta)
		require.NoError(t, err)
	}
	parent, err := r.CreateSession(ctx, schema.SessionMeta{GeneratorMeta: schema.GeneratorMeta{Model: types.Ptr("model")}})
	require.NoError(t, err)

	// An event which is worth telling the person is pending, with its
	// message, what it's for, and when it was due, and fired
	event := uuid.New()
	fired := time.Date(2026, 11, 6, 18, 0, 5, 0, time.UTC)
	response, err := r.RunEvent(ctx, event, schema.RunEventRequest{Session: parent.ID, Input: input, Title: "Call", Scheduled: fired.Add(-5 * time.Second), Fired: fired}, nil)
	require.NoError(t, err)
	require.NotNil(t, response.Event)
	pending := *response.Event
	assert.Equal(t, event, pending.Run)
	assert.Equal(t, parent.ID, pending.Session)
	assert.Equal(t, "Call", pending.Title)
	assert.Equal(t, "Call your mother", pending.Message)
	assert.Equal(t, schema.ImportanceAlert, pending.Importance)
	assert.Equal(t, schema.EventPending, pending.Status)
	assert.Equal(t, fired, pending.Fired)

	// One which isn't is ignored
	response, err = r.RunEvent(ctx, event, schema.RunEventRequest{Session: parent.ID, Input: input, Title: "Call"}, nil)
	require.NoError(t, err)
	require.NotNil(t, response.Event)
	assert.Equal(t, schema.EventIgnored, response.Event.Status)
	assert.Equal(t, "Nothing new", response.Event.Message)

	// Listed, the most recent first, and by session, and status
	list, err := r.ListEvents(ctx, schema.EventListRequest{})
	require.NoError(t, err)
	require.Equal(t, uint(2), list.Count)
	assert.Equal(t, response.Event.ID, list.Body[0].ID)
	list, err = r.ListEvents(ctx, schema.EventListRequest{Status: schema.EventPending})
	require.NoError(t, err)
	require.Equal(t, uint(1), list.Count)
	assert.Equal(t, pending.ID, list.Body[0].ID)
	list, err = r.ListEvents(ctx, schema.EventListRequest{Status: " pending "})
	require.NoError(t, err)
	assert.Equal(t, uint(1), list.Count, "a status, with space around it")
	list, err = r.ListEvents(ctx, schema.EventListRequest{Session: strings.ToUpper(parent.ID.String())})
	require.NoError(t, err)
	assert.Equal(t, uint(2), list.Count, "a session, in upper case")
	list, err = r.ListEvents(ctx, schema.EventListRequest{Session: uuid.New().String()})
	require.NoError(t, err)
	assert.Zero(t, list.Count)
	_, err = r.ListEvents(ctx, schema.EventListRequest{Session: "x"})
	assert.ErrorIs(t, err, llm.ErrBadParameter)
	_, err = r.ListEvents(ctx, schema.EventListRequest{Status: "lost"})
	assert.ErrorIs(t, err, llm.ErrBadParameter)

	// Got, and deleted
	got, err := r.GetEvent(ctx, pending.ID)
	require.NoError(t, err)
	assert.Equal(t, pending, *got)
	_, err = r.DeleteEvent(ctx, pending.ID)
	require.NoError(t, err)
	_, err = r.GetEvent(ctx, pending.ID)
	assert.ErrorIs(t, err, llm.ErrNotFound)
	_, err = r.DeleteEvent(ctx, pending.ID)
	assert.ErrorIs(t, err, llm.ErrNotFound)

	// Without an event store, the outcome isn't stored
	noStore, err := registry.New()
	require.NoError(t, err)
	_, err = noStore.ListEvents(ctx, schema.EventListRequest{})
	assert.ErrorIs(t, err, llm.ErrServiceUnavailable)
}
