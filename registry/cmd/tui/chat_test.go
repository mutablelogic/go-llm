package tui

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"uuid"

	// Packages
	textarea "github.com/charmbracelet/bubbles/textarea"
	viewport "github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	lipgloss "github.com/charmbracelet/lipgloss"
	llm "github.com/mutablelogic/go-llm"
	providerschema "github.com/mutablelogic/go-llm/provider/schema"
	schema "github.com/mutablelogic/go-llm/registry/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

// fakeClient replies to each question with the reply, streamed, or fails
type fakeClient struct {
	reply string
	err   error
}

func (c fakeClient) GetSession(context.Context, uuid.UUID) (*schema.Session, error) {
	return nil, errors.New("not used")
}

func (c fakeClient) ListSessionMessages(context.Context, uuid.UUID, schema.MessageListRequest) (*schema.MessageList, error) {
	return nil, errors.New("not used")
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
