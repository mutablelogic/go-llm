package registry_test

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"
	"uuid"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	providerschema "github.com/mutablelogic/go-llm/provider/schema"
	registry "github.com/mutablelogic/go-llm/registry"
	schema "github.com/mutablelogic/go-llm/registry/schema"
	test "github.com/mutablelogic/go-llm/registry/test"
	resourcemanager "github.com/mutablelogic/go-llm/resource"
	resourceschema "github.com/mutablelogic/go-llm/resource/schema"
	toolschema "github.com/mutablelogic/go-llm/toolkit/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

func Test_DeliverEvent(t *testing.T) {
	ctx := context.Background()

	// A registry with sessions, events, resources, and a function which is
	// given each reply
	sessions := test.NewSessionStore()
	events := test.NewEventStore()
	exists := func(ctx context.Context, id uuid.UUID) error {
		_, err := sessions.Get(ctx, id)
		return err
	}
	resources, err := resourcemanager.New(resourcemanager.NewStore(), resourcemanager.WithSessions(exists))
	require.NoError(t, err)
	var mu sync.Mutex
	var replies []string
	r, err := registry.New(registry.WithSessionStore(sessions), registry.WithEventStore(events), registry.WithResources(resources), registry.WithReplies(func(_ context.Context, session uuid.UUID, text string) {
		mu.Lock()
		defer mu.Unlock()
		replies = append(replies, session.String()+":"+text)
	}))
	require.NoError(t, err)
	r.SetDelivery(100*time.Millisecond, time.Minute, time.Minute, 10*time.Millisecond)
	require.NoError(t, r.Add("", newScripted()))
	meta := schema.SessionMeta{GeneratorMeta: schema.GeneratorMeta{Model: types.Ptr("model")}}
	parent, err := r.CreateSession(ctx, meta)
	require.NoError(t, err)
	run, err := r.CreateSession(ctx, meta)
	require.NoError(t, err)

	// The run made speech, which a tool returned
	speech, _, err := resources.Create(ctx, resourceschema.ResourceMeta{Name: "speech.wav", Type: "audio/wav", Session: &run.ID}, strings.NewReader("RIFF"))
	require.NoError(t, err)
	result, err := json.Marshal(toolschema.ResourcesResult{Resources: []toolschema.ResourceLink{{URI: speech.URI(), Name: "speech.wav", Type: "audio/wav"}}})
	require.NoError(t, err)
	messages := []providerschema.Message{
		{MessageMeta: providerschema.MessageMeta{Sender: "tool", Content: []providerschema.ContentBlock{{ToolResult: &providerschema.ToolResult{Name: "llm:speak", Content: result}}}}},
		{MessageMeta: providerschema.MessageMeta{Sender: "tool", Content: []providerschema.ContentBlock{{ToolResult: &providerschema.ToolResult{Name: "llm:speak", Content: result}}}}},
	}
	event, err := events.Create(ctx, schema.EventMeta{Run: run.ID, Session: parent.ID, Title: "Morning briefing", Message: "Good morning, it's sunny", Importance: schema.ImportanceNotify, Status: schema.EventPending})
	require.NoError(t, err)

	// While the session is replying, and until it's been idle for a while
	// since, it waits
	replied := r.Replying(parent.ID)
	done := make(chan error, 1)
	go func() { done <- r.Deliver(ctx, event, messages) }()
	time.Sleep(150 * time.Millisecond)
	list, err := r.ListSessionMessages(ctx, parent.ID, schema.MessageListRequest{})
	require.NoError(t, err)
	assert.Zero(t, list.Count, "it waits for the reply")
	replied()
	idle := time.Now()
	require.NoError(t, <-done)
	assert.GreaterOrEqual(t, time.Since(idle), 100*time.Millisecond, "it waits for the session to be idle")

	// It's the assistant's message, marked as the event's, with the speech,
	// once, copied to the session, and it's delivered, and given to the
	// function, as a reply is
	list, err = r.ListSessionMessages(ctx, parent.ID, schema.MessageListRequest{})
	require.NoError(t, err)
	require.Len(t, list.Body, 1)
	message := list.Body[0]
	assert.Equal(t, "assistant", message.Role())
	assert.Equal(t, "Good morning, it's sunny", message.Text())
	assert.Equal(t, &providerschema.Event{ID: event.ID, Title: "Morning briefing"}, message.Event)
	require.Len(t, message.Content, 2)
	require.NotNil(t, message.Content[1].Resource)
	assert.Equal(t, speech.URI(), message.Content[1].Resource.URL.String())
	assert.Equal(t, "audio/wav", message.Content[1].Resource.ContentType)
	copied, err := resources.Get(ctx, resourceschema.ResourceKey{Hash: speech.Hash, Session: &parent.ID})
	require.NoError(t, err)
	assert.Equal(t, "speech.wav", copied.Name)
	got, err := r.GetEvent(ctx, event.ID)
	require.NoError(t, err)
	assert.Equal(t, schema.EventDelivered, got.Status)
	assert.Equal(t, []string{parent.ID.String() + ":Good morning, it's sunny"}, replies)

	// An outcome which isn't pending isn't delivered
	ignored, err := events.Create(ctx, schema.EventMeta{Run: run.ID, Session: parent.ID, Message: "Nothing new", Importance: schema.ImportanceIgnore, Status: schema.EventIgnored})
	require.NoError(t, err)
	require.NoError(t, r.Deliver(ctx, ignored, nil))
	list, err = r.ListSessionMessages(ctx, parent.ID, schema.MessageListRequest{})
	require.NoError(t, err)
	assert.Len(t, list.Body, 1)

	// One for a session which was deleted isn't, and stays pending
	gone, err := events.Create(ctx, schema.EventMeta{Run: run.ID, Session: uuid.New(), Message: "Hello?", Importance: schema.ImportanceNotify, Status: schema.EventPending})
	require.NoError(t, err)
	assert.ErrorIs(t, r.Deliver(ctx, gone, nil), llm.ErrNotFound)
	got, err = r.GetEvent(ctx, gone.ID)
	require.NoError(t, err)
	assert.Equal(t, schema.EventPending, got.Status)
}

func Test_DeliverWhileReplying(t *testing.T) {
	ctx := context.Background()

	// A session whose model takes a while to reply
	sessions := test.NewSessionStore()
	events := test.NewEventStore()
	r, err := registry.New(registry.WithSessionStore(sessions), registry.WithEventStore(events), registry.WithToolkit(test.NewToolkit(t)))
	require.NoError(t, err)
	r.SetDelivery(time.Hour, 20*time.Millisecond, time.Minute, 5*time.Millisecond)
	p := newScripted(text("Here's the answer"))
	p.delay = 200 * time.Millisecond
	require.NoError(t, r.Add("", p))
	session, err := r.CreateSession(ctx, schema.SessionMeta{GeneratorMeta: schema.GeneratorMeta{Model: types.Ptr("model")}})
	require.NoError(t, err)
	event, err := events.Create(ctx, schema.EventMeta{Run: uuid.New(), Session: session.ID, Title: "Reminder", Message: "Call Anne", Importance: schema.ImportanceNotify, Status: schema.EventPending})
	require.NoError(t, err)

	// An event's outcome, which stops waiting for the session to be idle, is
	// added as
	// the model replies, and the reply is stored, after it, rather than
	// refused, as its conversation is as it was
	done := make(chan error, 1)
	go func() {
		time.Sleep(50 * time.Millisecond)
		done <- r.Deliver(ctx, event, nil)
	}()
	_, err = r.Chat(ctx, session.ID, []llm.Message{user(t, "A question")}, nil)
	require.NoError(t, err)
	require.NoError(t, <-done)
	list, err := r.ListSessionMessages(ctx, session.ID, schema.MessageListRequest{})
	require.NoError(t, err)
	require.Len(t, list.Body, 3)
	assert.Equal(t, "A question", list.Body[0].Text())
	assert.Equal(t, "Here's the answer", list.Body[1].Text())
	assert.Equal(t, "Call Anne", list.Body[2].Text())
	assert.NotNil(t, list.Body[2].Event)
}

func Test_DeliverAfterIdle(t *testing.T) {
	ctx := context.Background()

	sessions := test.NewSessionStore()
	events := test.NewEventStore()
	r, err := registry.New(registry.WithSessionStore(sessions), registry.WithEventStore(events), registry.WithToolkit(test.NewToolkit(t)))
	require.NoError(t, err)
	r.SetDelivery(200*time.Millisecond, time.Minute, time.Minute, 5*time.Millisecond)
	require.NoError(t, r.Add("", newScripted(text("Hello"))))
	session, err := r.CreateSession(ctx, schema.SessionMeta{GeneratorMeta: schema.GeneratorMeta{Model: types.Ptr("model")}})
	require.NoError(t, err)
	event, err := events.Create(ctx, schema.EventMeta{Run: uuid.New(), Session: session.ID, Message: "Call Anne", Importance: schema.ImportanceNotify, Status: schema.EventPending})
	require.NoError(t, err)

	// A message which was just added, and someone typing, keep it waiting,
	// until the session's been idle for long enough
	_, err = r.Chat(ctx, session.ID, []llm.Message{user(t, "Hi")}, nil)
	require.NoError(t, err)
	start := time.Now()
	done := make(chan error, 1)
	go func() { done <- r.Deliver(ctx, event, nil) }()
	time.Sleep(150 * time.Millisecond)
	_, err = r.SetSessionStatus(ctx, session.ID, schema.SessionTyping)
	require.NoError(t, err)
	time.Sleep(50 * time.Millisecond)
	_, err = r.SetSessionStatus(ctx, session.ID, schema.SessionIdle)
	require.NoError(t, err)
	typed := time.Now()
	require.NoError(t, <-done)
	assert.GreaterOrEqual(t, time.Since(typed), 180*time.Millisecond, "idle for long enough after typing, within a poll")
	assert.GreaterOrEqual(t, time.Since(start), 380*time.Millisecond)
	list, err := r.ListSessionMessages(ctx, session.ID, schema.MessageListRequest{})
	require.NoError(t, err)
	require.Len(t, list.Body, 3)
	assert.Equal(t, "Call Anne", list.Body[2].Text())
}

func Test_DeliverAlert(t *testing.T) {
	ctx := context.Background()
	sessions := test.NewSessionStore()
	events := test.NewEventStore()
	r, err := registry.New(registry.WithSessionStore(sessions), registry.WithEventStore(events), registry.WithToolkit(test.NewToolkit(t)))
	require.NoError(t, err)
	r.SetDelivery(time.Hour, time.Hour, time.Minute, 5*time.Millisecond)
	require.NoError(t, r.Add("", newScripted(text("Hello"))))
	session, err := r.CreateSession(ctx, schema.SessionMeta{GeneratorMeta: schema.GeneratorMeta{Model: types.Ptr("model")}})
	require.NoError(t, err)
	_, err = r.Chat(ctx, session.ID, []llm.Message{user(t, "Hi")}, nil)
	require.NoError(t, err)

	// An alert doesn't wait for the session to have been idle for a while,
	// only for its reply
	alert, err := events.Create(ctx, schema.EventMeta{Run: uuid.New(), Session: session.ID, Message: "Leave now for the train", Importance: schema.ImportanceAlert, Status: schema.EventPending})
	require.NoError(t, err)
	replied := r.Replying(session.ID)
	done := make(chan error, 1)
	go func() { done <- r.Deliver(ctx, alert, nil) }()
	time.Sleep(50 * time.Millisecond)
	select {
	case <-done:
		t.Fatal("delivered as the session was replying")
	default:
	}
	replied()
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(time.Second):
		t.Fatal("an alert waited for the session to have been idle")
	}
	list, err := r.ListSessionMessages(ctx, session.ID, schema.MessageListRequest{})
	require.NoError(t, err)
	require.Len(t, list.Body, 3)
	assert.Equal(t, "Leave now for the train", list.Body[2].Text())
}
