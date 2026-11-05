package tui

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
	"uuid"

	// Packages
	textarea "github.com/charmbracelet/bubbles/textarea"
	viewport "github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	lipgloss "github.com/charmbracelet/lipgloss"
	llm "github.com/mutablelogic/go-llm"
	eventschema "github.com/mutablelogic/go-llm/broadcaster/schema"
	memoryschema "github.com/mutablelogic/go-llm/memory/schema"
	providerschema "github.com/mutablelogic/go-llm/provider/schema"
	schema "github.com/mutablelogic/go-llm/registry/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

// fakeClient replies to each question with the reply, streamed, or fails,
// has the stored session, when it's set, and records the statuses set
type fakeClient struct {
	reply    string
	err      error
	stored   *schema.Session
	statuses *[]schema.SessionStatus
	details  *memoryschema.Details
}

func (c fakeClient) GetDetails(context.Context) (*memoryschema.Details, error) {
	if c.details == nil {
		return nil, errors.New("no memory")
	}
	return c.details, nil
}

func (c fakeClient) GetSession(context.Context, uuid.UUID) (*schema.Session, error) {
	if c.stored == nil {
		return nil, errors.New("not used")
	}
	session := *c.stored
	session.Messages = nil
	return &session, nil
}

func (c fakeClient) ListSessionMessages(context.Context, uuid.UUID, schema.MessageListRequest) (*schema.MessageList, error) {
	if c.stored == nil {
		return nil, errors.New("not used")
	}
	return &schema.MessageList{Count: uint(len(c.stored.Messages)), Body: c.stored.Messages}, nil
}

func (c fakeClient) SetSessionStatus(_ context.Context, id uuid.UUID, status schema.SessionStatus) (*schema.Session, error) {
	if c.statuses != nil {
		*c.statuses = append(*c.statuses, status)
	}
	return &schema.Session{ID: id, Status: status}, nil
}

func (c fakeClient) WatchEvents(context.Context, func(eventschema.Event) error) error {
	return errors.New("not used")
}

func (c fakeClient) Chat(_ context.Context, id uuid.UUID, req schema.ChatRequest, stream llm.GeneratorStreamFn) (*schema.ChatResponse, error) {
	if c.err != nil {
		return nil, c.err
	}
	stream(llm.StreamText, c.reply)
	reply := providerschema.Message{MessageMeta: providerschema.MessageMeta{Sender: "assistant", Content: []providerschema.ContentBlock{{Text: types.Ptr(c.reply)}}}, Result: providerschema.ResultStop}
	return &schema.ChatResponse{ID: id, Serial: 2, SessionResponse: schema.SessionResponse{
		Messages: []providerschema.Message{{MessageMeta: req.Messages[0]}, reply},
		Usage:    schema.SessionUsage{Usage: llm.Usage{InputTokens: 10, OutputTokens: 5}, Input: 4, Output: 5, Overhead: 6},
	}}, nil
}

// newTestModel returns the interface's model for the session, with the
// client, whose streamed pieces are collected
func newTestModel(client Client, session *schema.Session) (*model, *[]tea.Msg) {
	var sent []tea.Msg
	m := &model{ctx: context.Background(), client: client, session: session, history: viewport.New(80, 20), input: textarea.New()}
	m.send = func(msg tea.Msg) { sent = append(sent, msg) }
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	return m, &sent
}

// ask types the question, sends it, and returns the reply's message
func ask(t *testing.T, m *model, question string) tea.Msg {
	t.Helper()
	m.input.SetValue(question)
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	require.NotNil(t, cmd)
	assert.True(t, m.waiting)
	assert.Contains(t, m.history.View(), question)
	return cmd()
}

func Test_Chat(t *testing.T) {
	session := func() *schema.Session {
		return &schema.Session{ID: uuid.New(), Serial: 1, SessionMeta: schema.SessionMeta{GeneratorMeta: schema.GeneratorMeta{Model: types.Ptr("eliza:eliza-1966-en")}},
			Messages: []providerschema.Message{{MessageMeta: providerschema.MessageMeta{Sender: "user", Content: []providerschema.ContentBlock{{Text: types.Ptr("Earlier question")}}}}}}
	}

	t.Run("History", func(t *testing.T) {
		// The session's messages, and its model, are shown
		m, _ := newTestModel(fakeClient{}, session())
		view := m.View()
		assert.Contains(t, view, "Earlier question")
		assert.Contains(t, view, "eliza:eliza-1966-en")
	})

	t.Run("Reply", func(t *testing.T) {
		// The question is sent, the reply streamed, and then the session has
		// both, as they were stored, with its serial and usage
		m, sent := newTestModel(fakeClient{reply: "How do you do?"}, session())
		reply := ask(t, m, "Hello")
		require.Len(t, *sent, 1)
		m.Update((*sent)[0])
		assert.Contains(t, m.history.View(), "How do you do?")

		m.Update(reply)
		assert.False(t, m.waiting)
		assert.Empty(t, m.streamed)
		require.Len(t, m.session.Messages, 3)
		assert.Equal(t, "Hello", m.session.Messages[1].Text())
		assert.Equal(t, "How do you do?", m.session.Messages[2].Text())
		assert.Equal(t, uint64(2), m.session.Serial)
		assert.Equal(t, uint(15), m.session.Usage.Input+m.session.Usage.Output+m.session.Usage.Overhead)
		assert.Contains(t, m.View(), "15 tokens")

		// A second reply adds its usage to the session's
		m.Update(ask(t, m, "Again"))
		assert.Equal(t, llm.Usage{InputTokens: 20, OutputTokens: 10}, m.session.Usage.Usage)
		assert.Len(t, m.session.Messages, 5)
	})

	t.Run("Error", func(t *testing.T) {
		// A question which isn't sent is typed again, and the error shown
		m, _ := newTestModel(fakeClient{err: errors.New("unavailable")}, session())
		m.Update(ask(t, m, "Hello"))
		assert.False(t, m.waiting)
		assert.Len(t, m.session.Messages, 1)
		assert.Equal(t, "Hello", m.input.Value())
		assert.True(t, strings.Contains(m.history.View(), "unavailable"))
	})

	t.Run("Empty", func(t *testing.T) {
		// Nothing typed isn't sent
		m, _ := newTestModel(fakeClient{}, session())
		_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
		assert.Nil(t, cmd)
		assert.False(t, m.waiting)
	})

	t.Run("Quit", func(t *testing.T) {
		m, _ := newTestModel(fakeClient{}, session())
		_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
		require.NotNil(t, cmd)
		assert.IsType(t, tea.QuitMsg{}, cmd())
	})
}

func Test_AddPiece(t *testing.T) {
	// Pieces of the same kind are added to their group, a change of kind is
	// a new group, and log lines are each their own
	var groups []*group
	for _, p := range []piece{
		{llm.StreamThinking, "Let me "}, {llm.StreamThinking, "think.\n"},
		{llm.StreamText, "\nThe answer "}, {llm.StreamText, "is 42."},
		{llm.StreamLog, "tool: fetching"}, {llm.StreamLog, "tool: done"},
		{llm.StreamText, "Done."},
	} {
		groups = addPiece(groups, p)
	}
	var got []piece
	for _, g := range groups {
		got = append(got, piece{g.what, g.text.String()})
	}
	assert.Equal(t, []piece{
		{llm.StreamThinking, "Let me think.\n"},
		{llm.StreamText, "\nThe answer is 42."},
		{llm.StreamLog, "tool: fetching"},
		{llm.StreamLog, "tool: done"},
		{llm.StreamText, "Done."},
	}, got)
}

func Test_Draft(t *testing.T) {
	// A question which isn't sent doesn't replace the next one, which has
	// been started, and is shown with the error
	m, _ := newTestModel(fakeClient{err: errors.New("unavailable")}, &schema.Session{ID: uuid.New()})
	reply := ask(t, m, "First question")
	m.input.SetValue("Next question")
	m.Update(reply)
	assert.Equal(t, "Next question", m.input.Value())
	assert.Contains(t, m.history.View(), "First question")
	assert.Contains(t, m.history.View(), "unavailable")
}

func Test_Scroll(t *testing.T) {
	// The history follows the reply as it's streamed, unless it's been
	// scrolled up, when it stays where it is
	var messages []providerschema.Message
	for i := range 50 {
		messages = append(messages, providerschema.Message{MessageMeta: providerschema.MessageMeta{Sender: "user", Content: []providerschema.ContentBlock{{Text: types.Ptr(fmt.Sprint("Question ", i))}}}})
	}
	m, _ := newTestModel(fakeClient{}, &schema.Session{ID: uuid.New(), Messages: messages})
	assert.True(t, m.history.AtBottom())

	m.waiting = true
	m.Update(streamMsg{llm.StreamText, "Streaming"})
	assert.True(t, m.history.AtBottom())

	m.Update(tea.KeyMsg{Type: tea.KeyPgUp})
	offset := m.history.YOffset
	require.False(t, m.history.AtBottom())
	m.Update(streamMsg{llm.StreamText, " more"})
	assert.Equal(t, offset, m.history.YOffset)
}

func Test_Wrap(t *testing.T) {
	// What's streamed, and the user's messages, are wrapped to the width,
	// and the thinking and the text are on lines of their own
	long := strings.Repeat("word ", 40)
	m, _ := newTestModel(fakeClient{}, &schema.Session{ID: uuid.New(), Messages: []providerschema.Message{
		{MessageMeta: providerschema.MessageMeta{Sender: "user", Content: []providerschema.ContentBlock{{Text: types.Ptr(long)}}}},
	}})
	m.Update(tea.WindowSizeMsg{Width: 40, Height: 100})
	m.debug = true
	m.waiting = true
	m.Update(streamMsg{llm.StreamThinking, "Thinking " + long})
	m.Update(streamMsg{llm.StreamText, "Text " + long})

	view := m.history.View()
	for _, line := range strings.Split(view, "\n") {
		assert.LessOrEqual(t, lipgloss.Width(line), 40, line)
	}
	var thinking, text int
	for i, line := range strings.Split(view, "\n") {
		if strings.Contains(line, "Thinking") {
			thinking = i
		}
		if strings.Contains(line, "Text") {
			text = i
		}
	}
	assert.Greater(t, text, thinking+1, "the text starts on a line of its own, after the thinking")
}

// sessionChange returns the event of a change to the session, with the data
func sessionChange(t *testing.T, id uuid.UUID, data schema.SessionEvent) eventMsg {
	t.Helper()
	raw, err := json.Marshal(data)
	require.NoError(t, err)
	return eventMsg{Action: eventschema.EventUpdate, Schema: eventSchema, Table: sessionTable, Row: id.String(), Data: raw}
}

func Test_Changes(t *testing.T) {
	says := func(role, text string) providerschema.Message {
		return providerschema.Message{MessageMeta: providerschema.MessageMeta{Sender: role, Content: []providerschema.ContentBlock{{Text: types.Ptr(text)}}}}
	}

	t.Run("Elsewhere", func(t *testing.T) {
		// A change which this window hasn't seen, such as a reply in another
		// window, loads the session again
		id := uuid.New()
		stored := &schema.Session{ID: id, Serial: 2, Messages: []providerschema.Message{says("user", "Hi"), says("assistant", "Hello from elsewhere")}}
		m, _ := newTestModel(fakeClient{stored: stored}, &schema.Session{ID: id, Serial: 1})
		_, cmd := m.Update(sessionChange(t, id, schema.SessionEvent{Serial: 1}))
		assert.Nil(t, cmd, "a change which has been seen isn't loaded")
		_, cmd = m.Update(sessionChange(t, id, schema.SessionEvent{Serial: 2}))
		require.NotNil(t, cmd)
		m.Update(cmd())
		assert.Equal(t, uint64(2), m.session.Serial)
		assert.Contains(t, m.history.View(), "Hello from elsewhere")
	})

	t.Run("Status", func(t *testing.T) {
		// The status is the session's, as the server says
		id := uuid.New()
		m, _ := newTestModel(fakeClient{}, &schema.Session{ID: id})
		m.Update(sessionChange(t, id, schema.SessionEvent{Status: schema.SessionReplying}))
		assert.Contains(t, m.status(), "replying…")
		m.Update(sessionChange(t, id, schema.SessionEvent{Status: schema.SessionTyping}))
		assert.Contains(t, m.status(), "typing…")
		m.Update(sessionChange(t, id, schema.SessionEvent{Status: schema.SessionIdle}))
		assert.NotContains(t, m.status(), "…")
	})

	t.Run("Typing", func(t *testing.T) {
		// The server is told someone is typing, as they type, at most every
		// typingInterval, and the status is shown as the server says
		var statuses []schema.SessionStatus
		id := uuid.New()
		m, _ := newTestModel(fakeClient{statuses: &statuses}, &schema.Session{ID: id})
		m.input.Focus()
		for _, key := range "Hi" {
			_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{key}})
			for _, msg := range run(cmd) {
				m.Update(msg)
			}
		}
		assert.Equal(t, []schema.SessionStatus{schema.SessionTyping}, statuses)
		m.Update(sessionChange(t, id, schema.SessionEvent{Status: schema.SessionTyping}))
		assert.Contains(t, m.status(), "typing…")
	})

	t.Run("OwnReply", func(t *testing.T) {
		// The change which this window's reply makes isn't loaded, but one
		// made elsewhere as it was waited for is, once it's stored
		id := uuid.New()
		m, _ := newTestModel(fakeClient{reply: "Hello", stored: &schema.Session{ID: id, Serial: 3}}, &schema.Session{ID: id, Serial: 1})
		reply := ask(t, m, "Hi")
		_, cmd := m.Update(sessionChange(t, id, schema.SessionEvent{Serial: 2}))
		assert.Nil(t, cmd)
		_, cmd = m.Update(reply)
		assert.Nil(t, cmd, "the change was the reply's")

		reply = ask(t, m, "Again")
		m.Update(sessionChange(t, id, schema.SessionEvent{Serial: 4}))
		_, cmd = m.Update(reply)
		assert.NotNil(t, cmd, "a change was made elsewhere")
	})

	t.Run("Deleted", func(t *testing.T) {
		id := uuid.New()
		m, _ := newTestModel(fakeClient{}, &schema.Session{ID: id})
		m.Update(eventMsg{Action: eventschema.EventDelete, Schema: eventSchema, Table: sessionTable, Row: id.String()})
		assert.Contains(t, m.history.View(), "the session was deleted")
	})
}

// run runs the command, and the commands it batches, and returns their
// messages
func run(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	switch msg := cmd().(type) {
	case tea.BatchMsg:
		var result []tea.Msg
		for _, cmd := range msg {
			result = append(result, run(cmd)...)
		}
		return result
	case nil:
		return nil
	default:
		return []tea.Msg{msg}
	}
}

// flakyClient fails to watch the changes the times, and then watches them,
// sending the event, until the context is cancelled
type flakyClient struct {
	fakeClient
	mu    sync.Mutex
	fails int
	event eventschema.Event
}

func (c *flakyClient) WatchEvents(ctx context.Context, fn func(eventschema.Event) error) error {
	c.mu.Lock()
	fail := c.fails > 0
	c.fails--
	c.mu.Unlock()
	if fail {
		return errors.New("server is stopping")
	}
	fn(c.event)
	<-ctx.Done()
	return nil
}

func Test_Watch(t *testing.T) {
	// When the changes can't be watched, that's sent, and they're watched
	// again after a wait, which is longer each time, and once they are, the
	// interface is told, so it loads the session again
	retry, max, settled := watchRetry, watchRetryMax, watchSettled
	watchRetry, watchRetryMax, watchSettled = 10*time.Millisecond, 25*time.Millisecond, 50*time.Millisecond
	t.Cleanup(func() { watchRetry, watchRetryMax, watchSettled = retry, max, settled })

	id := uuid.New()
	event := eventschema.Event{Action: eventschema.EventUpdate, Schema: eventSchema, Table: sessionTable, Row: id.String()}
	client := &flakyClient{fails: 3, event: event}
	ctx, cancel := context.WithCancel(context.Background())
	events := make(chan tea.Msg, 16)
	watching := make(chan struct{})
	go func() { watch(ctx, client, id, events); close(watching) }()
	defer func() { cancel(); <-watching }()

	var got []tea.Msg
	for len(got) < 5 {
		select {
		case msg := <-events:
			got = append(got, msg)
		case <-time.After(5 * time.Second):
			t.Fatalf("only %d messages", len(got))
		}
	}
	for i, retry := range []time.Duration{10 * time.Millisecond, 20 * time.Millisecond, 25 * time.Millisecond} {
		require.IsType(t, watchedMsg{}, got[i])
		assert.Equal(t, retry, got[i].(watchedMsg).retry)
		assert.ErrorContains(t, got[i].(watchedMsg).err, "stopping")
	}
	assert.Equal(t, eventMsg(event), got[3])
	assert.Equal(t, rewatchedMsg{}, got[4])

	// The interface shows why, and clears it, and loads the session, once
	// the changes are watched again
	m, _ := newTestModel(fakeClient{}, &schema.Session{ID: id})
	m.Update(got[0])
	assert.Contains(t, m.history.View(), "server is stopping")
	_, cmd := m.Update(got[4])
	assert.NotNil(t, cmd)
	assert.NotContains(t, m.history.View(), "server is stopping")
}

func Test_Logs(t *testing.T) {
	// What tools log, which is streamed, and isn't stored, is kept, and
	// shown before the tools' results, once the reply is complete
	id := uuid.New()
	m, _ := newTestModel(fakeClient{}, &schema.Session{ID: id})
	m.debug = true
	m.waiting = true
	m.session.Messages = append(m.session.Messages, providerschema.Message{MessageMeta: providerschema.MessageMeta{Sender: "user", Content: []providerschema.ContentBlock{{Text: types.Ptr("Lights on")}}}})
	m.Update(streamMsg{llm.StreamLog, "call_service: Calling service domain=light service=turn_on"})
	m.Update(streamMsg{llm.StreamText, "Done"})
	call := providerschema.Message{MessageMeta: providerschema.MessageMeta{Sender: "assistant", Content: []providerschema.ContentBlock{{ToolCall: &providerschema.ToolCall{Name: "call_service", Input: json.RawMessage(`{}`)}}}}}
	result := providerschema.Message{MessageMeta: providerschema.MessageMeta{Sender: "tool", Content: []providerschema.ContentBlock{{ToolResult: &providerschema.ToolResult{Name: "call_service", Content: json.RawMessage(`[]`)}}}}}
	reply := providerschema.Message{MessageMeta: providerschema.MessageMeta{Sender: "assistant", Content: []providerschema.ContentBlock{{Text: types.Ptr("Done")}}}}
	m.Update(replyMsg{asked: 0, response: &schema.ChatResponse{ID: id, Serial: 2, SessionResponse: schema.SessionResponse{
		Messages: []providerschema.Message{m.session.Messages[0], call, result, reply},
	}}})
	view := m.history.View()
	logged := strings.Index(view, "Calling service")
	require.GreaterOrEqual(t, logged, 0, view)
	assert.Less(t, strings.Index(view, "→ call_service"), logged)
	assert.Less(t, logged, strings.Index(view, "← call_service"))
}

func Test_Debug(t *testing.T) {
	// Without debugging, the model's thinking is shown as that it thought,
	// and the tools it calls, and their results, aren't shown, and with it,
	// they're shown, by their canonical names
	thought := providerschema.Message{MessageMeta: providerschema.MessageMeta{Sender: "assistant", Content: []providerschema.ContentBlock{
		{Thinking: types.Ptr("Secret reasoning")},
		{ToolCall: &providerschema.ToolCall{Name: "homeassistant__call_service", Input: json.RawMessage(`{}`)}},
	}}}
	result := providerschema.Message{MessageMeta: providerschema.MessageMeta{Sender: "tool", Content: []providerschema.ContentBlock{
		{ToolResult: &providerschema.ToolResult{Name: "homeassistant__call_service", Content: json.RawMessage(`"ok"`)}},
	}}}
	reply := providerschema.Message{MessageMeta: providerschema.MessageMeta{Sender: "assistant", Content: []providerschema.ContentBlock{{Text: types.Ptr("The lamp is off")}}}}
	session := &schema.Session{ID: uuid.New(), Messages: []providerschema.Message{thought, result, reply}}

	m, _ := newTestModel(fakeClient{}, session)
	view := m.history.View()
	assert.Contains(t, view, "Thinking…")
	assert.NotContains(t, view, "Secret reasoning")
	assert.NotContains(t, view, "call_service")
	assert.Contains(t, view, "The lamp is off")
	m.waiting = true
	m.Update(streamMsg{llm.StreamThinking, "Streamed reasoning"})
	assert.NotContains(t, m.history.View(), "Streamed reasoning")

	m, _ = newTestModel(fakeClient{}, session)
	m.debug = true
	m.render()
	view = m.history.View()
	assert.Contains(t, view, "Secret reasoning")
	assert.Contains(t, view, "→ homeassistant:call_service")
	assert.Contains(t, view, "← homeassistant:call_service")
}

func Test_Reloads(t *testing.T) {
	says := func(text string) providerschema.Message {
		return providerschema.Message{MessageMeta: providerschema.MessageMeta{Sender: "user", Content: []providerschema.ContentBlock{{Text: types.Ptr(text)}}}}
	}

	t.Run("Older", func(t *testing.T) {
		// A session which is older than the one shown isn't shown
		id := uuid.New()
		m, _ := newTestModel(fakeClient{}, &schema.Session{ID: id, Serial: 3, Messages: []providerschema.Message{says("Newer")}})
		m.loading = true
		m.Update(loadedMsg{session: &schema.Session{ID: id, Serial: 2, Messages: []providerschema.Message{says("Older")}}})
		assert.Equal(t, uint64(3), m.session.Serial)
		assert.Contains(t, m.history.View(), "Newer")
	})

	t.Run("Skipped", func(t *testing.T) {
		// A reply stored after a change made elsewhere, which wasn't shown,
		// loads the session again
		id := uuid.New()
		m, _ := newTestModel(fakeClient{reply: "Hello"}, &schema.Session{ID: id, Serial: 1})
		reply := ask(t, m, "Hi").(replyMsg)
		reply.response.Serial = 3
		_, cmd := m.Update(reply)
		assert.NotNil(t, cmd)
	})

	t.Run("Status", func(t *testing.T) {
		// A status which changes as the session is loaded loads it again
		id := uuid.New()
		m, _ := newTestModel(fakeClient{}, &schema.Session{ID: id, Serial: 1})
		m.loading = true
		m.Update(sessionChange(t, id, schema.SessionEvent{Status: schema.SessionIdle}))
		_, cmd := m.Update(loadedMsg{session: &schema.Session{ID: id, Serial: 1, Status: schema.SessionReplying}})
		assert.NotNil(t, cmd, "loaded again")
	})
}

func Test_WatchFirst(t *testing.T) {
	// The session is loaded once the changes are watched, the first time
	// too, as a change may have been made as they started to be
	settled := watchSettled
	watchSettled = 10 * time.Millisecond
	t.Cleanup(func() { watchSettled = settled })
	id := uuid.New()
	event := eventschema.Event{Action: eventschema.EventUpdate, Schema: eventSchema, Table: sessionTable, Row: id.String()}
	ctx, cancel := context.WithCancel(context.Background())
	events := make(chan tea.Msg, 16)
	watching := make(chan struct{})
	go func() { watch(ctx, &flakyClient{event: event}, id, events); close(watching) }()
	defer func() { cancel(); <-watching }()
	assert.Equal(t, eventMsg(event), <-events)
	select {
	case msg := <-events:
		assert.Equal(t, rewatchedMsg{}, msg)
	case <-time.After(5 * time.Second):
		t.Fatal("not loaded once watched")
	}
}

func Test_LogsReloaded(t *testing.T) {
	// What tools logged stays before their results when the session is
	// loaded again with messages from elsewhere before them
	id := uuid.New()
	says := func(role, text string) providerschema.Message {
		return providerschema.Message{MessageMeta: providerschema.MessageMeta{Sender: role, Content: []providerschema.ContentBlock{{Text: types.Ptr(text)}}}}
	}
	question := says("user", "Lights on")
	call := providerschema.Message{MessageMeta: providerschema.MessageMeta{Sender: "assistant", Content: []providerschema.ContentBlock{{ToolCall: &providerschema.ToolCall{ID: "call-1", Name: "call_service", Input: json.RawMessage(`{}`)}}}}}
	result := providerschema.Message{MessageMeta: providerschema.MessageMeta{Sender: "tool", Content: []providerschema.ContentBlock{{ToolResult: &providerschema.ToolResult{ID: "call-1", Name: "call_service", Content: json.RawMessage(`[]`)}}}}}
	reply := says("assistant", "Done")
	m, _ := newTestModel(fakeClient{}, &schema.Session{ID: id, Serial: 1})
	m.debug = true
	m.waiting = true
	m.session.Messages = []providerschema.Message{question}
	m.Update(streamMsg{llm.StreamLog, "Asking Home Assistant to turn on the lights"})
	m.Update(replyMsg{asked: 0, response: &schema.ChatResponse{ID: id, Serial: 3, SessionResponse: schema.SessionResponse{
		Messages: []providerschema.Message{question, call, result, reply},
	}}})

	// Another window's turn is before this one, when it's loaded again
	m.Update(loadedMsg{session: &schema.Session{ID: id, Serial: 3, Messages: []providerschema.Message{
		says("user", "Elsewhere"), says("assistant", "Hello from elsewhere"), question, call, result, reply,
	}}})
	view := m.history.View()
	logged := strings.Index(view, "turn on the lights")
	require.GreaterOrEqual(t, logged, 0, view)
	assert.Less(t, strings.Index(view, "Hello from elsewhere"), logged)
	assert.Less(t, strings.Index(view, "→ call_service"), logged)
	assert.Less(t, logged, strings.Index(view, "← call_service"))
}

func Test_Progress(t *testing.T) {
	// A tool's progress is shown as the reply is streamed, in place of the
	// one before, and not once the reply is complete
	id := uuid.New()
	m, _ := newTestModel(fakeClient{}, &schema.Session{ID: id})
	m.waiting = true
	m.Update(streamMsg{llm.StreamProgress, llm.Progress{Progress: 1, Message: "Agent segment is thinking"}.Text()})
	assert.Contains(t, m.history.View(), "Agent segment is thinking")
	m.Update(streamMsg{llm.StreamProgress, llm.Progress{Progress: 3, Total: 4, Message: "Segmenting"}.Text()})
	view := m.history.View()
	assert.Contains(t, view, "Segmenting (75%)")
	assert.NotContains(t, view, "Agent segment is thinking")
	m.Update(replyMsg{asked: 0, response: &schema.ChatResponse{ID: id, Serial: 2}})
	assert.NotContains(t, m.history.View(), "Segmenting")
}

func Test_Labels(t *testing.T) {
	id := uuid.New()
	newSession := func(model string) *schema.Session {
		return &schema.Session{ID: id, SessionMeta: schema.SessionMeta{GeneratorMeta: schema.GeneratorMeta{Model: types.Ptr(model)}}, Messages: []providerschema.Message{
			{MessageMeta: providerschema.MessageMeta{Sender: "user", Content: []providerschema.ContentBlock{{Text: types.Ptr("Hello")}}}},
			{MessageMeta: providerschema.MessageMeta{Sender: "assistant", Content: []providerschema.ContentBlock{{Text: types.Ptr("Hi")}}}},
		}}
	}
	detail := func(action eventschema.EventAction, row, data string) detailMsg {
		event := eventschema.Event{Action: action, Schema: memorySchema, Table: detailTable, Row: row}
		if data != "" {
			event.Data = json.RawMessage(data)
		}
		return detailMsg(event)
	}

	t.Run("Unknown", func(t *testing.T) {
		// The person is user, and the assistant is its model's provider, or
		// assistant, without one
		m, _ := newTestModel(fakeClient{}, newSession("deepseek:deepseek-chat"))
		assert.Equal(t, "You", m.userLabel())
		assert.Equal(t, "deepseek", m.assistantLabel())
		m, _ = newTestModel(fakeClient{}, &schema.Session{ID: id})
		assert.Equal(t, "assistant", m.assistantLabel())
	})

	t.Run("Names", func(t *testing.T) {
		// The names, as they're loaded, and as they change
		m, _ := newTestModel(fakeClient{}, newSession("deepseek:deepseek-chat"))
		m.Update(detailsMsg{details: &memoryschema.Details{Name: &memoryschema.Detail{Value: "David"}, Assistant: &memoryschema.Detail{Value: "Nano"}}})
		assert.Equal(t, "David", m.userLabel())
		assert.Equal(t, "Nano", m.assistantLabel())
		view := m.history.View()
		assert.Contains(t, view, "David")
		assert.Contains(t, view, "Nano")

		m.Update(detail(eventschema.EventUpdate, "assistant", `{"value":"Robin"}`))
		assert.Contains(t, m.history.View(), "Robin")
		m.Update(detail(eventschema.EventDelete, "assistant", ""))
		assert.Equal(t, "deepseek", m.assistantLabel())

		// A declined name isn't used, and other details, or a memory which
		// can't be loaded, change nothing
		m.Update(detail(eventschema.EventUpdate, "name", `{"declined":true}`))
		assert.Equal(t, "You", m.userLabel())
		m.Update(detail(eventschema.EventCreate, "location", `{"value":"Berlin"}`))
		m.Update(detail(eventschema.EventCreate, "unknown", `{"value":"x"}`))
		m.Update(detailsMsg{err: errors.New("no memory")})
		assert.Equal(t, "You", m.userLabel())
		assert.Equal(t, "Berlin", m.details.Location.Value)
	})
}

func Test_StatusFollowsSession(t *testing.T) {
	// A change to the session elsewhere, such as its model, is loaded, and
	// shown in the status line
	id := uuid.New()
	before := &schema.Session{ID: id, Serial: 1, SessionMeta: schema.SessionMeta{GeneratorMeta: schema.GeneratorMeta{Model: types.Ptr("eliza:eliza-1966-en")}}}
	after := &schema.Session{ID: id, Serial: 2, SessionMeta: schema.SessionMeta{GeneratorMeta: schema.GeneratorMeta{Model: types.Ptr("deepseek:deepseek-chat")}}}
	m, _ := newTestModel(fakeClient{stored: after}, before)
	assert.Contains(t, m.status(), "eliza:eliza-1966-en")

	cmd := m.changed(eventschema.Event{Action: eventschema.EventUpdate, Schema: eventSchema, Table: sessionTable, Row: id.String(), Data: json.RawMessage(`{"serial":2}`)})
	require.NotNil(t, cmd)
	m.Update(cmd())
	assert.Contains(t, m.status(), "deepseek:deepseek-chat")
}

func Test_Indent(t *testing.T) {
	// What's under a label is indented, and wrapped to fit the window
	long := strings.Repeat("word ", 40)
	m, _ := newTestModel(fakeClient{}, &schema.Session{ID: uuid.New(), Messages: []providerschema.Message{
		{MessageMeta: providerschema.MessageMeta{Sender: "user", Content: []providerschema.ContentBlock{{Text: types.Ptr(long)}}}},
		{MessageMeta: providerschema.MessageMeta{Sender: "assistant", Content: []providerschema.ContentBlock{{Thinking: types.Ptr("Hmm")}, {Text: types.Ptr("Hi")}}}},
	}})
	m.logs = map[string][]string{positionKey(1): {"Remembering your name: David"}}
	m.render()
	content := m.history.View()
	assert.Contains(t, content, "\n  word word")
	assert.Contains(t, content, "\n  "+thinking)
	assert.Contains(t, content, "\n  Remembering your name: David")
	for _, line := range strings.Split(content, "\n") {
		assert.LessOrEqual(t, lipgloss.Width(line), 80, line)
	}
}

func Test_TurnLabel(t *testing.T) {
	// A reply which calls a tool, and then replies, is under one label,
	// with what the tool logged under it, and the next question has its own
	call := providerschema.ContentBlock{ToolCall: &providerschema.ToolCall{ID: "1", Name: "ollama__web_search"}}
	result := providerschema.ContentBlock{ToolResult: &providerschema.ToolResult{ID: "1", Name: "ollama__web_search"}}
	message := func(sender string, content ...providerschema.ContentBlock) providerschema.Message {
		return providerschema.Message{MessageMeta: providerschema.MessageMeta{Sender: sender, Content: content}}
	}
	m, _ := newTestModel(fakeClient{}, &schema.Session{ID: uuid.New(), Messages: []providerschema.Message{
		message("user", providerschema.ContentBlock{Text: types.Ptr("Pools?")}),
		message("assistant", providerschema.ContentBlock{Text: types.Ptr("Let me search.")}, call),
		message("tool", result),
		message("assistant", providerschema.ContentBlock{Text: types.Ptr("Neukölln.")}),
		message("user", providerschema.ContentBlock{Text: types.Ptr("Thanks")}),
	}})
	m.details = memoryschema.Details{Assistant: &memoryschema.Detail{Value: "Omodie"}}
	m.logs = map[string][]string{resultKey("1"): {"Searching the web"}}
	m.history.Height = 100
	m.render()
	content := m.history.View()
	assert.Equal(t, 1, strings.Count(content, "Omodie"))
	assert.Equal(t, 2, strings.Count(content, "You"))
	assert.Less(t, strings.Index(content, "Omodie"), strings.Index(content, "Searching the web"))
	assert.Less(t, strings.Index(content, "Searching the web"), strings.Index(content, "Neukölln."))
}

func Test_LogsUnderLabel(t *testing.T) {
	// A reply which only calls a tool, so has nothing to show, and then
	// replies, has its label above what the tool logged
	message := func(sender string, content ...providerschema.ContentBlock) providerschema.Message {
		return providerschema.Message{MessageMeta: providerschema.MessageMeta{Sender: sender, Content: content}}
	}
	m, _ := newTestModel(fakeClient{}, &schema.Session{ID: uuid.New(), SessionMeta: schema.SessionMeta{GeneratorMeta: schema.GeneratorMeta{Model: types.Ptr("openai:gpt-5")}}, Messages: []providerschema.Message{
		message("user", providerschema.ContentBlock{Text: types.Ptr("I'd rather not say my name")}),
		message("assistant", providerschema.ContentBlock{ToolCall: &providerschema.ToolCall{ID: "1", Name: "memory__set_detail"}}),
		message("tool", providerschema.ContentBlock{ToolResult: &providerschema.ToolResult{ID: "1", Name: "memory__set_detail"}}),
		message("assistant", providerschema.ContentBlock{Text: types.Ptr("No problem.")}),
	}})
	m.logs = map[string][]string{resultKey("1"): {"Not remembering your name, as you'd rather not say"}}
	m.history.Height = 100
	m.render()
	content := m.history.View()
	assert.Equal(t, 1, strings.Count(content, "openai"))
	assert.Less(t, strings.Index(content, "openai"), strings.Index(content, "Not remembering"))
	assert.Less(t, strings.Index(content, "Not remembering"), strings.Index(content, "No problem."))
}
