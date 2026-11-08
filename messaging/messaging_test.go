package messaging_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
	"uuid"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	messaging "github.com/mutablelogic/go-llm/messaging"
	schema "github.com/mutablelogic/go-llm/messaging/schema"
	toolkit "github.com/mutablelogic/go-llm/toolkit"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

///////////////////////////////////////////////////////////////////////////////
// FAKE PROVIDER

// fake is a provider whose messages are sent to it by the test, and which
// keeps what it's sent
type fake struct {
	name     string
	incoming chan messaging.Message
	mu       sync.Mutex
	sent     map[string][]string
	ids      int
}

func newFake(name string) *fake {
	return &fake{name: name, incoming: make(chan messaging.Message), sent: make(map[string][]string)}
}

func (f *fake) Name() string { return f.name }

func (f *fake) Link(code string) string { return "https://example.com/" + f.name + "?start=" + code }

func (f *fake) Receive(ctx context.Context, fn func(context.Context, messaging.Message) error) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case m := <-f.incoming:
			_ = fn(ctx, m)
		}
	}
}

func (f *fake) Send(_ context.Context, address, text string) ([]messaging.Sent, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sent[address] = append(f.sent[address], text)
	f.ids++
	return []messaging.Sent{{ID: strconv.Itoa(f.ids), Text: true}}, nil
}

func (f *fake) messages(address string) []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.sent[address]...)
}

// code returns the code of a link
func code(t *testing.T, link string) string {
	t.Helper()
	u, err := url.Parse(link)
	require.NoError(t, err)
	return u.Query().Get("start")
}

// session is a session, for the tools
type session uuid.UUID

func (s session) ID() uuid.UUID                   { return uuid.UUID(s) }
func (session) Logger() *slog.Logger              { return slog.New(slog.DiscardHandler) }
func (session) Progress(float64, float64, string) {}
func (session) Usage(llm.Usage)                   {}

///////////////////////////////////////////////////////////////////////////////
// TESTS

func Test_Messaging_Connect(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	provider := newFake("fake")
	chats := make(chan string, 4)
	m, err := messaging.New(messaging.NewStore(), messaging.WithProvider(provider), messaging.WithChat(func(_ context.Context, channel schema.Channel, message messaging.Message) error {
		chats <- channel.Session.String() + ":" + message.Text
		return nil
	}))
	require.NoError(t, err)
	go m.Run(ctx, nil)

	id := uuid.New()
	link, expires, err := m.Connect(ctx, id, "")
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(link, "https://example.com/fake?start="))
	assert.WithinDuration(t, time.Now().Add(messaging.CodeExpiry), expires, time.Minute)

	// A message from a chat which isn't connected is ignored
	provider.incoming <- messaging.Message{Address: "1", Text: "hello"}

	// The link connects the chat, once
	provider.incoming <- messaging.Message{Address: "1", Title: "David", Code: code(t, link), Text: "/start"}
	provider.incoming <- messaging.Message{Address: "2", Code: code(t, link), Text: "/start"}
	assert.Eventually(t, func() bool { return len(provider.messages("2")) == 1 }, time.Second, 10*time.Millisecond)
	assert.Equal(t, []string{"Connected. Your messages here now go to your assistant, and its replies come here."}, provider.messages("1"))
	assert.Contains(t, provider.messages("2")[0], "expired")

	channels, err := m.List(ctx, id)
	require.NoError(t, err)
	require.Len(t, channels, 1)
	assert.Equal(t, "fake", channels[0].Provider)
	assert.Equal(t, "1", channels[0].Address)
	assert.Equal(t, "David", channels[0].Title)

	// A message from the connected chat is sent to its session
	provider.incoming <- messaging.Message{Address: "1", Text: "hello"}
	select {
	case got := <-chats:
		assert.Equal(t, id.String()+":hello", got)
	case <-time.After(time.Second):
		t.Fatal("message not sent to the session")
	}
	select {
	case got := <-chats:
		t.Fatalf("unexpected message %q", got)
	default:
	}
}

func Test_Messaging_Reply(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	provider := newFake("fake")
	m, err := messaging.New(messaging.NewStore(), messaging.WithProvider(provider))
	require.NoError(t, err)
	go m.Run(ctx, nil)

	// Two chats, connected to one session
	id := uuid.New()
	for _, address := range []string{"1", "2"} {
		link, _, err := m.Connect(ctx, id, "fake")
		require.NoError(t, err)
		provider.incoming <- messaging.Message{Address: address, Code: code(t, link)}
	}
	require.Eventually(t, func() bool {
		return len(provider.messages("1")) == 1 && len(provider.messages("2")) == 1
	}, time.Second, 10*time.Millisecond)

	// A reply goes to both, and a session without channels is fine
	require.NoError(t, m.Reply(ctx, id, messaging.Outgoing{Text: "It's sunny"}))
	require.NoError(t, m.Reply(ctx, uuid.New(), messaging.Outgoing{Text: "Nobody hears this"}))
	assert.Equal(t, "It's sunny", provider.messages("1")[1])
	assert.Equal(t, "It's sunny", provider.messages("2")[1])

	// Send to one channel
	channels, err := m.List(ctx, id)
	require.NoError(t, err)
	sent, err := m.Send(ctx, id, messaging.Outgoing{Text: "Just you"}, channels[0].ID)
	require.NoError(t, err)
	assert.Len(t, sent, 1)

	// Another session's channel can't be sent to
	_, err = m.Send(ctx, uuid.New(), messaging.Outgoing{Text: "No"}, channels[0].ID)
	assert.ErrorIs(t, err, llm.ErrNotFound)

	// Disconnect it, and delete the session's others
	_, err = m.Disconnect(ctx, channels[0].ID)
	require.NoError(t, err)
	n, err := m.DeleteSession(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, 1, n)
	channels, err = m.List(ctx, uuid.Nil())
	require.NoError(t, err)
	assert.Empty(t, channels)
}

func Test_Messaging_Move(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	provider := newFake("fake")
	m, err := messaging.New(messaging.NewStore(), messaging.WithProvider(provider))
	require.NoError(t, err)
	go m.Run(ctx, nil)

	// A chat connected to a second session moves to it
	first, second := uuid.New(), uuid.New()
	for _, id := range []uuid.UUID{first, second} {
		link, _, err := m.Connect(ctx, id, "")
		require.NoError(t, err)
		provider.incoming <- messaging.Message{Address: "1", Code: code(t, link)}
	}
	assert.Eventually(t, func() bool { return len(provider.messages("1")) == 2 }, time.Second, 10*time.Millisecond)
	channels, err := m.List(ctx, uuid.Nil())
	require.NoError(t, err)
	require.Len(t, channels, 1)
	assert.Equal(t, second, channels[0].Session)
}

func Test_Messaging_Errors(t *testing.T) {
	ctx := context.Background()
	_, err := messaging.New(nil)
	assert.ErrorIs(t, err, llm.ErrBadParameter)
	_, err = messaging.New(messaging.NewStore(), messaging.WithProvider(newFake("a")), messaging.WithProvider(newFake("a")))
	assert.ErrorIs(t, err, llm.ErrConflict)

	m, err := messaging.New(messaging.NewStore(), messaging.WithProvider(newFake("a")), messaging.WithProvider(newFake("b")))
	require.NoError(t, err)
	assert.Equal(t, []string{"a", "b"}, m.Providers())
	_, _, err = m.Connect(ctx, uuid.New(), "")
	assert.ErrorIs(t, err, llm.ErrBadParameter)
	_, _, err = m.Connect(ctx, uuid.New(), "c")
	assert.ErrorIs(t, err, llm.ErrNotFound)
	_, _, err = m.Connect(ctx, uuid.Nil(), "a")
	assert.ErrorIs(t, err, llm.ErrBadParameter)
	_, err = m.Send(ctx, uuid.New(), messaging.Outgoing{Text: ""})
	assert.ErrorIs(t, err, llm.ErrBadParameter)
	_, err = m.Send(ctx, uuid.New(), messaging.Outgoing{Text: "hello"})
	assert.ErrorIs(t, err, llm.ErrNotFound)
}

func Test_Messaging_Tools(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	provider := newFake("fake")
	m, err := messaging.New(messaging.NewStore(), messaging.WithProvider(provider))
	require.NoError(t, err)
	go m.Run(ctx, nil)
	assert.Contains(t, m.SystemPrompt(), "(fake)")

	tools := make(map[string]llm.Tool)
	for _, tool := range m.Tools() {
		tools[tool.Name()] = tool
	}
	require.Len(t, tools, 4)

	// Without a session, the tools fail
	_, err = tools["list"].Run(ctx, nil)
	assert.ErrorIs(t, err, llm.ErrBadParameter)

	// Connect, in a session
	id := uuid.New()
	sctx := toolkit.WithSession(ctx, session(id))
	result, err := tools["connect"].Run(sctx, json.RawMessage(`{}`))
	require.NoError(t, err)
	link := toMap(t, result)["link"].(string)
	provider.incoming <- messaging.Message{Address: "1", Title: "David", Code: code(t, link)}
	assert.Eventually(t, func() bool { return len(provider.messages("1")) == 1 }, time.Second, 10*time.Millisecond)

	// List
	result, err = tools["list"].Run(sctx, nil)
	require.NoError(t, err)
	list := toMap(t, result)
	assert.EqualValues(t, 1, list["count"])
	channel := list["channels"].([]any)[0].(map[string]any)
	assert.Equal(t, "David", channel["chat"])

	// Send
	_, err = tools["send"].Run(sctx, json.RawMessage(`{"text":"Hello from the assistant"}`))
	require.NoError(t, err)
	assert.Equal(t, "Hello from the assistant", provider.messages("1")[1])

	// Another session can't disconnect it, and this one can
	_, err = tools["disconnect"].Run(toolkit.WithSession(ctx, session(uuid.New())), json.RawMessage(`{"id":"`+channel["id"].(string)+`"}`))
	assert.ErrorIs(t, err, llm.ErrNotFound)
	_, err = tools["disconnect"].Run(sctx, json.RawMessage(`{"id":"`+channel["id"].(string)+`"}`))
	require.NoError(t, err)
	_, err = tools["send"].Run(sctx, json.RawMessage(`{"text":"Anyone?"}`))
	assert.True(t, errors.Is(err, llm.ErrNotFound))
}

// toMap returns the result, as JSON decodes it
func toMap(t *testing.T, v any) map[string]any {
	t.Helper()
	data, err := json.Marshal(v)
	require.NoError(t, err)
	var result map[string]any
	require.NoError(t, json.Unmarshal(data, &result))
	return result
}

func Test_Messaging_DeletedSession(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	provider := newFake("fake")
	var mu sync.Mutex
	deleted := map[uuid.UUID]bool{}
	exists := func(_ context.Context, id uuid.UUID) error {
		mu.Lock()
		defer mu.Unlock()
		if deleted[id] {
			return llm.ErrNotFound.Withf("session %q", id)
		}
		return nil
	}
	m, err := messaging.New(messaging.NewStore(), messaging.WithProvider(provider), messaging.WithSessions(exists), messaging.WithChat(func(context.Context, schema.Channel, messaging.Message) error {
		return llm.ErrNotFound.With("model not found")
	}))
	require.NoError(t, err)
	go m.Run(ctx, nil)

	// A chat connected to a session
	id := uuid.New()
	link, _, err := m.Connect(ctx, id, "")
	require.NoError(t, err)
	provider.incoming <- messaging.Message{Address: "1", Code: code(t, link)}
	require.Eventually(t, func() bool { return len(provider.messages("1")) == 1 }, time.Second, 10*time.Millisecond)

	// A message which can't be answered, as the model isn't found, keeps the
	// channel, as its session exists
	provider.incoming <- messaging.Message{Address: "1", Text: "hello"}
	require.Eventually(t, func() bool { return len(provider.messages("1")) == 2 }, time.Second, 10*time.Millisecond)
	assert.Contains(t, provider.messages("1")[1], "model not found")
	channels, err := m.List(ctx, id)
	require.NoError(t, err)
	assert.Len(t, channels, 1)

	// Once the session is deleted, a link made before can't connect a chat
	link, _, err = m.Connect(ctx, id, "")
	require.NoError(t, err)
	mu.Lock()
	deleted[id] = true
	mu.Unlock()
	provider.incoming <- messaging.Message{Address: "2", Code: code(t, link)}
	require.Eventually(t, func() bool { return len(provider.messages("2")) == 1 }, time.Second, 10*time.Millisecond)
	assert.Contains(t, provider.messages("2")[0], "expired")

	// and a message from its channel disconnects it
	provider.incoming <- messaging.Message{Address: "1", Text: "hello?"}
	require.Eventually(t, func() bool { return len(provider.messages("1")) == 3 }, time.Second, 10*time.Millisecond)
	assert.Contains(t, provider.messages("1")[2], "deleted")
	channels, err = m.List(ctx, uuid.Nil())
	require.NoError(t, err)
	assert.Empty(t, channels)
}

func Test_Messaging_DeleteSessionForgetsLinks(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	provider := newFake("fake")
	m, err := messaging.New(messaging.NewStore(), messaging.WithProvider(provider))
	require.NoError(t, err)
	go m.Run(ctx, nil)

	// A link for a session which is deleted, with no channels, is forgotten
	id := uuid.New()
	link, _, err := m.Connect(ctx, id, "")
	require.NoError(t, err)
	n, err := m.DeleteSession(ctx, id)
	require.NoError(t, err)
	assert.Zero(t, n)
	provider.incoming <- messaging.Message{Address: "1", Code: code(t, link)}
	require.Eventually(t, func() bool { return len(provider.messages("1")) == 1 }, time.Second, 10*time.Millisecond)
	assert.Contains(t, provider.messages("1")[0], "expired")
}

// fileFake is a provider which can send files, which keeps them, with
// their captions
type fileFake struct {
	*fake
	files []string
}

func (f *fileFake) SendFile(_ context.Context, address string, file messaging.File, open func() (io.ReadCloser, error), caption string) ([]messaging.Sent, error) {
	content, err := open()
	if err != nil {
		return nil, err
	}
	defer content.Close()
	data, err := io.ReadAll(content)
	if err != nil {
		return nil, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.files = append(f.files, address+":"+file.Name+":"+string(data)+":"+caption)
	f.ids++
	return []messaging.Sent{{ID: strconv.Itoa(f.ids), File: file.Name, Caption: caption != ""}}, nil
}

func Test_Messaging_Files(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// The session's files
	id := uuid.New()
	files := map[string]messaging.File{
		"resource:///image": {URI: "resource:///image", Name: "chart.png", Type: "image/png"},
		"resource:///audio": {URI: "resource:///audio", Name: "speech.ogg", Type: "audio/ogg"},
	}
	open := func(_ context.Context, session uuid.UUID, uri string) (messaging.File, io.ReadCloser, error) {
		file, ok := files[uri]
		if !ok || session != id {
			return messaging.File{}, nil, llm.ErrNotFound.Withf("file %q", uri)
		}
		return file, io.NopCloser(strings.NewReader(file.Name)), nil
	}
	connect := func(t *testing.T, m *messaging.Messaging, provider *fake) {
		t.Helper()
		go m.Run(ctx, nil)
		link, _, err := m.Connect(ctx, id, "")
		require.NoError(t, err)
		provider.incoming <- messaging.Message{Address: "1", Code: code(t, link)}
		require.Eventually(t, func() bool { return len(provider.messages("1")) == 1 }, time.Second, 10*time.Millisecond)
	}

	t.Run("Caption", func(t *testing.T) {
		// The text is an image's caption, when it's first
		provider := &fileFake{fake: newFake("fake")}
		m, err := messaging.New(messaging.NewStore(), messaging.WithProvider(provider), messaging.WithFiles(open))
		require.NoError(t, err)
		connect(t, m, provider.fake)
		require.NoError(t, m.Reply(ctx, id, messaging.Outgoing{Text: "Here it is", Files: []string{"resource:///image", "resource:///audio"}}))
		assert.Len(t, provider.messages("1"), 1, "no message, other than the one it was connected with")
		assert.Equal(t, []string{"1:chart.png:chart.png:Here it is", "1:speech.ogg:speech.ogg:"}, provider.files)
	})

	t.Run("Text first", func(t *testing.T) {
		// The text is sent before a file which isn't an image, and a file
		// which can't be opened is an error, after the others are sent
		provider := &fileFake{fake: newFake("fake")}
		m, err := messaging.New(messaging.NewStore(), messaging.WithProvider(provider), messaging.WithFiles(open))
		require.NoError(t, err)
		connect(t, m, provider.fake)
		err = m.Reply(ctx, id, messaging.Outgoing{Text: "Listen", Files: []string{"resource:///audio", "resource:///gone"}})
		assert.ErrorIs(t, err, llm.ErrNotFound)
		assert.Equal(t, "Listen", provider.messages("1")[1])
		assert.Equal(t, []string{"1:speech.ogg:speech.ogg:"}, provider.files)

		// and the text is sent, when no file can be opened
		require.Error(t, m.Reply(ctx, id, messaging.Outgoing{Text: "Still here", Files: []string{"resource:///gone"}}))
		assert.Equal(t, "Still here", provider.messages("1")[2])

		// Files alone are sent, without text
		require.NoError(t, m.Reply(ctx, id, messaging.Outgoing{Files: []string{"resource:///image"}}))
		assert.Len(t, provider.files, 2)
		assert.Len(t, provider.messages("1"), 3)
	})

	t.Run("Receipts", func(t *testing.T) {
		// What's sent is each message, in order, by its identifier, with
		// what it was, and a file which couldn't be sent is an error of the
		// channel, beside what was
		provider := &fileFake{fake: newFake("fake")}
		m, err := messaging.New(messaging.NewStore(), messaging.WithProvider(provider), messaging.WithFiles(open))
		require.NoError(t, err)
		connect(t, m, provider.fake)
		deliveries, err := m.Send(ctx, id, messaging.Outgoing{Text: "Listen", Files: []string{"resource:///audio", "resource:///gone", "resource:///image"}})
		require.NoError(t, err)
		require.Len(t, deliveries, 1)
		assert.Equal(t, []messaging.Sent{{ID: "2", Text: true}, {ID: "3", File: "speech.ogg"}, {ID: "4", File: "chart.png"}}, deliveries[0].Messages)
		assert.ErrorIs(t, deliveries[0].Err, llm.ErrNotFound)

		// The tool says so too, with a count of each chat's messages
		tools := make(map[string]llm.Tool)
		for _, tool := range m.Tools() {
			tools[tool.Name()] = tool
		}
		result, err := tools["send"].Run(toolkit.WithSession(ctx, session(id)), json.RawMessage(`{"text":"Look","files":["resource:///image","resource:///gone"]}`))
		require.NoError(t, err, "something was sent")
		output := toMap(t, result)
		sent := output["sent"].([]any)[0].(map[string]any)
		assert.EqualValues(t, 1, sent["count"])
		assert.Equal(t, []any{map[string]any{"id": "5", "file": "chart.png", "caption": true}}, sent["messages"])
		assert.Contains(t, sent["error"], "resource:///gone")

		// and it's an error, when nothing was sent
		_, err = tools["send"].Run(toolkit.WithSession(ctx, session(id)), json.RawMessage(`{"files":["resource:///gone"]}`))
		assert.ErrorIs(t, err, llm.ErrNotFound)
	})

	t.Run("Text only", func(t *testing.T) {
		// A provider which can't send files is sent the text
		provider := newFake("fake")
		m, err := messaging.New(messaging.NewStore(), messaging.WithProvider(provider), messaging.WithFiles(open))
		require.NoError(t, err)
		connect(t, m, provider)
		require.NoError(t, m.Reply(ctx, id, messaging.Outgoing{Text: "Here it is", Files: []string{"resource:///image"}}))
		assert.Equal(t, "Here it is", provider.messages("1")[1])
		require.NoError(t, m.Reply(ctx, id, messaging.Outgoing{Files: []string{"resource:///image"}}))
		assert.Len(t, provider.messages("1"), 2)
	})
}

func Test_Messaging_Attachments(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	provider := newFake("fake")
	got := make(chan messaging.Message, 1)
	m, err := messaging.New(messaging.NewStore(), messaging.WithProvider(provider), messaging.WithChat(func(_ context.Context, _ schema.Channel, message messaging.Message) error {
		got <- message
		return nil
	}))
	require.NoError(t, err)
	go m.Run(ctx, nil)
	link, _, err := m.Connect(ctx, uuid.New(), "")
	require.NoError(t, err)
	provider.incoming <- messaging.Message{Address: "1", Code: code(t, link)}

	// A message with a file, and no text, such as a voice message, is sent
	// to the session
	voice := messaging.Attachment{Name: "voice.ogg", Type: "audio/ogg"}
	provider.incoming <- messaging.Message{Address: "1", Attachments: []messaging.Attachment{voice}}
	select {
	case message := <-got:
		require.Len(t, message.Attachments, 1)
		assert.Equal(t, "voice.ogg", message.Attachments[0].Name)
	case <-time.After(time.Second):
		t.Fatal("a message with only a file wasn't sent to the session")
	}
}
