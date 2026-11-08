package registry_test

import (
	"context"
	"net/url"
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
	var files []string
	r, err := registry.New(registry.WithSessionStore(sessions), registry.WithEventStore(events), registry.WithResources(resources), registry.WithReplies(func(_ context.Context, session uuid.UUID, reply registry.Reply) {
		mu.Lock()
		defer mu.Unlock()
		replies = append(replies, session.String()+":"+reply.Text)
		for _, link := range reply.Resources {
			files = append(files, link.URI+":"+link.Name+":"+link.Type)
		}
	}))
	require.NoError(t, err)
	r.SetDelivery(100*time.Millisecond, time.Minute, time.Minute, 10*time.Millisecond)
	require.NoError(t, r.Add("", newScripted()))
	meta := schema.SessionMeta{GeneratorMeta: schema.GeneratorMeta{Model: types.Ptr("model")}}
	parent, err := r.CreateSession(ctx, meta)
	require.NoError(t, err)
	run, err := r.CreateSession(ctx, meta)
	require.NoError(t, err)

	// An earlier run made a picture, and this one made speech, such as in
	// an agent it called, which its messages needn't refer to
	_, _, err = resources.Create(ctx, resourceschema.ResourceMeta{Name: "earlier.png", Type: "image/png", Session: &run.ID}, strings.NewReader("\x89PNG"))
	require.NoError(t, err)
	time.Sleep(5 * time.Millisecond)
	started := time.Now()
	speech, _, err := resources.Create(ctx, resourceschema.ResourceMeta{Name: "speech.wav", Type: "audio/wav", Session: &run.ID}, strings.NewReader("RIFF"))
	require.NoError(t, err)
	event, err := events.Create(ctx, schema.EventMeta{Run: run.ID, Session: parent.ID, Title: "Morning briefing", Message: "Good morning, it's sunny", Importance: schema.ImportanceNotify, Status: schema.EventPending})
	require.NoError(t, err)

	// While the session is replying, and until it's been idle for a while
	// since, it waits
	replied := r.Replying(parent.ID)
	done := make(chan error, 1)
	go func() { done <- r.Deliver(ctx, event, started) }()
	time.Sleep(150 * time.Millisecond)
	list, err := r.ListSessionMessages(ctx, parent.ID, schema.MessageListRequest{})
	require.NoError(t, err)
	assert.Zero(t, list.Count, "it waits for the reply")
	replied()
	time.Sleep(40 * time.Millisecond)
	list, err = r.ListSessionMessages(ctx, parent.ID, schema.MessageListRequest{})
	require.NoError(t, err)
	assert.Zero(t, list.Count, "it waits for the session to have been idle")
	require.NoError(t, <-done)

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
	assert.Equal(t, []string{speech.URI() + ":speech.wav:audio/wav"}, files, "with the files it made")

	// Delivering it again, such as when its status wasn't changed, doesn't
	// add it again, nor give it to the function
	require.NoError(t, r.Deliver(ctx, event, started))
	list, err = r.ListSessionMessages(ctx, parent.ID, schema.MessageListRequest{})
	require.NoError(t, err)
	assert.Len(t, list.Body, 1)
	assert.Len(t, replies, 1)

	// An outcome which isn't pending isn't delivered
	ignored, err := events.Create(ctx, schema.EventMeta{Run: run.ID, Session: parent.ID, Message: "Nothing new", Importance: schema.ImportanceIgnore, Status: schema.EventIgnored})
	require.NoError(t, err)
	require.NoError(t, r.Deliver(ctx, ignored, started))
	list, err = r.ListSessionMessages(ctx, parent.ID, schema.MessageListRequest{})
	require.NoError(t, err)
	assert.Len(t, list.Body, 1)

	// One for a session which was deleted isn't, and stays pending
	gone, err := events.Create(ctx, schema.EventMeta{Run: run.ID, Session: uuid.New(), Message: "Hello?", Importance: schema.ImportanceNotify, Status: schema.EventPending})
	require.NoError(t, err)
	assert.ErrorIs(t, r.Deliver(ctx, gone, started), llm.ErrNotFound)
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
		done <- r.Deliver(ctx, event, time.Now())
	}()
	_, err = r.Chat(ctx, session.ID, []llm.Message{user(t, "A question")}, func(llm.StreamType, string) {})
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
	go func() { done <- r.Deliver(ctx, event, time.Now()) }()
	time.Sleep(150 * time.Millisecond)
	_, err = r.SetSessionStatus(ctx, session.ID, schema.SessionTyping)
	require.NoError(t, err)
	time.Sleep(50 * time.Millisecond)
	_, err = r.SetSessionStatus(ctx, session.ID, schema.SessionIdle)
	require.NoError(t, err)
	time.Sleep(100 * time.Millisecond)
	select {
	case <-done:
		t.Fatal("delivered before the session had been idle for long enough, after typing")
	default:
	}
	require.NoError(t, <-done)
	assert.GreaterOrEqual(t, time.Since(start), 300*time.Millisecond)
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
	go func() { done <- r.Deliver(ctx, alert, time.Now()) }()
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

func Test_ChatReplyResources(t *testing.T) {
	ctx := context.Background()

	// A registry with resources, whose model replies with an image it made
	sessions := test.NewSessionStore()
	exists := func(ctx context.Context, id uuid.UUID) error {
		_, err := sessions.Get(ctx, id)
		return err
	}
	resources, err := resourcemanager.New(resourcemanager.NewStore(), resourcemanager.WithSessions(exists))
	require.NoError(t, err)
	var got []registry.Reply
	r, err := registry.New(registry.WithSessionStore(sessions), registry.WithResources(resources), registry.WithToolkit(test.NewToolkit(t)), registry.WithReplies(func(_ context.Context, _ uuid.UUID, reply registry.Reply) {
		got = append(got, reply)
	}))
	require.NoError(t, err)
	made := &providerschema.Message{MessageMeta: providerschema.MessageMeta{Sender: "assistant", Content: []providerschema.ContentBlock{
		{Text: types.Ptr("Here's a cat")},
		{Resource: &providerschema.Resource{ResourceMeta: providerschema.ResourceMeta{ContentType: "image/png", Data: []byte("\x89PNG\r\n\x1a\n")}}},
	}}, Result: providerschema.ResultStop}
	require.NoError(t, r.Add("", newScripted(made)))
	session, err := r.CreateSession(ctx, schema.SessionMeta{GeneratorMeta: schema.GeneratorMeta{Model: types.Ptr("model")}})
	require.NoError(t, err)

	// The person attaches a picture of their own, which is linked to the
	// session as they chat, but isn't one the reply made
	theirs, _, err := resources.Create(ctx, resourceschema.ResourceMeta{Name: "dog.png", Type: "image/png"}, strings.NewReader("\x89PNG dog"))
	require.NoError(t, err)
	u, err := url.Parse(theirs.URI())
	require.NoError(t, err)
	question := &providerschema.Message{MessageMeta: providerschema.MessageMeta{Sender: "user", Content: []providerschema.ContentBlock{
		{Resource: &providerschema.Resource{ResourceMeta: providerschema.ResourceMeta{ContentType: "image/png", URL: u}}},
		{Text: types.Ptr("Draw a cat, like this dog")},
	}}}

	// The reply is given with the image it made, which is the session's
	_, err = r.Chat(ctx, session.ID, []llm.Message{question}, nil)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "Here's a cat", got[0].Text)
	require.Len(t, got[0].Resources, 1)
	assert.Equal(t, "image/png", got[0].Resources[0].Type)
	hash, err := resourceschema.ParseURI(got[0].Resources[0].URI)
	require.NoError(t, err)
	_, err = resources.Get(ctx, resourceschema.ResourceKey{Hash: hash, Session: &session.ID})
	require.NoError(t, err)
}

func Test_RetryDeliveries(t *testing.T) {
	ctx := context.Background()
	sessions := test.NewSessionStore()
	events := test.NewEventStore()
	var mu sync.Mutex
	var replies []string
	r, err := registry.New(registry.WithSessionStore(sessions), registry.WithEventStore(events), registry.WithReplies(func(_ context.Context, _ uuid.UUID, reply registry.Reply) {
		mu.Lock()
		defer mu.Unlock()
		replies = append(replies, reply.Text)
	}))
	require.NoError(t, err)
	r.SetDelivery(0, time.Minute, time.Minute, 5*time.Millisecond)
	require.NoError(t, r.Add("", newScripted()))
	session, err := r.CreateSession(ctx, schema.SessionMeta{GeneratorMeta: schema.GeneratorMeta{Model: types.Ptr("model")}})
	require.NoError(t, err)

	// Outcomes which are pending, which weren't delivered, such as when the
	// server stopped, are, and those which aren't pending aren't
	pending, err := events.Create(ctx, schema.EventMeta{Run: uuid.New(), Session: session.ID, Message: "Call Anne", Importance: schema.ImportanceNotify, Status: schema.EventPending})
	require.NoError(t, err)
	_, err = events.Create(ctx, schema.EventMeta{Run: uuid.New(), Session: session.ID, Message: "Nothing", Importance: schema.ImportanceIgnore, Status: schema.EventIgnored})
	require.NoError(t, err)
	r.RetryDeliveries(ctx)
	require.Eventually(t, func() bool {
		got, err := r.GetEvent(ctx, pending.ID)
		return err == nil && got.Status == schema.EventDelivered
	}, time.Second, 5*time.Millisecond)
	list, err := r.ListSessionMessages(ctx, session.ID, schema.MessageListRequest{})
	require.NoError(t, err)
	require.Len(t, list.Body, 1)
	assert.Equal(t, "Call Anne", list.Body[0].Text())
	mu.Lock()
	assert.Equal(t, []string{"Call Anne"}, replies)
	mu.Unlock()
}

func Test_ForgottenDuringReply(t *testing.T) {
	ctx := context.Background()
	sessions := test.NewSessionStore()
	events := test.NewEventStore()
	r, err := registry.New(registry.WithSessionStore(sessions), registry.WithEventStore(events), registry.WithToolkit(test.NewToolkit(t)))
	require.NoError(t, err)
	r.SetDelivery(time.Hour, 0, time.Minute, 5*time.Millisecond)
	p := newScripted(text("An answer"))
	p.delay = 200 * time.Millisecond
	require.NoError(t, r.Add("", p))
	session, err := r.CreateSession(ctx, schema.SessionMeta{GeneratorMeta: schema.GeneratorMeta{Model: types.Ptr("model")}})
	require.NoError(t, err)
	event, err := events.Create(ctx, schema.EventMeta{Run: uuid.New(), Session: session.ID, Message: "Call Anne", Importance: schema.ImportanceNotify, Status: schema.EventPending})
	require.NoError(t, err)

	// The first reply, whose conversation is forgotten as it's written, and
	// then an event's outcome delivered, isn't stored, as the conversation
	// it's to was forgotten, though only a message was added since
	go func() {
		time.Sleep(50 * time.Millisecond)
		_, _ = r.ForgetSessionMessages(ctx, session.ID)
		_ = r.Deliver(ctx, event, time.Now())
	}()
	_, err = r.Chat(ctx, session.ID, []llm.Message{user(t, "A question")}, func(llm.StreamType, string) {})
	assert.ErrorIs(t, err, llm.ErrConflict)
	list, err := r.ListSessionMessages(ctx, session.ID, schema.MessageListRequest{})
	require.NoError(t, err)
	require.Len(t, list.Body, 1)
	assert.Equal(t, "Call Anne", list.Body[0].Text())
}
