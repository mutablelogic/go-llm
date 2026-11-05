// Package tui is a terminal interface to chat in a session, which is a
// throwaway, until there's a better one
package tui

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
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
	toolschema "github.com/mutablelogic/go-llm/toolkit/schema"
	servertui "github.com/mutablelogic/go-server/pkg/tui"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Client is what the interface chats in a session with, and watches the
// changes to it with, which others make, such as in another window, and
// gets the memory's details with, such as the names of the person and the
// assistant
type Client interface {
	GetDetails(ctx context.Context) (*memoryschema.Details, error)
	GetSession(ctx context.Context, id uuid.UUID) (*schema.Session, error)
	ListSessionMessages(ctx context.Context, id uuid.UUID, req schema.MessageListRequest) (*schema.MessageList, error)
	Chat(ctx context.Context, id uuid.UUID, req schema.ChatRequest, stream llm.GeneratorStreamFn) (*schema.ChatResponse, error)
	SetSessionStatus(ctx context.Context, id uuid.UUID, status schema.SessionStatus) (*schema.Session, error)
	WatchEvents(ctx context.Context, fn func(eventschema.Event) error) error
}

// model is the interface's state: the session, its messages, what's being
// streamed of the reply, what's typed, and the session's status, as others
// change it
type model struct {
	ctx      context.Context
	client   Client
	debug    bool // Whether the model's thinking, and the tools it calls, are shown
	session  *schema.Session
	details  memoryschema.Details // The memory's details, such as the names of the person and the assistant
	changes  uint64               // How many changes to the details have been applied, so a reload which they happened during is done again
	speaker  string               // Who was last labelled, as the history is rendered, so a turn has one label
	send     func(tea.Msg)
	history  viewport.Model
	input    textarea.Model
	width    int
	waiting  bool
	streamed []*group
	progress string              // A tool's progress, as the reply is streamed, which the next replaces
	logs     map[string][]string // What tools logged, by the tool result, or the position, it's shown before
	rendered bool
	err      error
	typed    time.Time // When the server was last told someone is typing here
	loading  bool      // Whether the session is being loaded again
	stale    bool      // Whether it's to be loaded again once it's loaded
	seen     uint64    // The latest serial of the session's changes
}

// piece is a piece of a reply as it's streamed
type piece struct {
	what llm.StreamType
	text string
}

// group is the pieces of a reply of the same kind, which follow each other,
// such as its thinking, and then its text, as they're streamed, which is
// added to as they are, rather than joined again each time
type group struct {
	what llm.StreamType
	text strings.Builder
}

// The messages of the interface
type (
	streamMsg piece
	replyMsg  struct {
		response *schema.ChatResponse
		err      error
		asked    int    // The messages before the question
		question string // The question, which is typed again when it isn't sent
	}
	eventMsg   eventschema.Event // A change to the session
	detailMsg  eventschema.Event // A change to the memory's details
	detailsMsg struct {
		details *memoryschema.Details
		err     error
		changes uint64 // The changes which had been applied when they were loaded
	}
	watchedMsg struct {
		err   error         // Why the changes aren't watched
		retry time.Duration // When they're watched again
	}
	rewatchedMsg struct{} // The changes are watched again
	loadedMsg    struct {
		session *schema.Session
		err     error
	}
)

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

var (
	youStyle     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
	modelStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("10"))
	dimStyle     = lipgloss.NewStyle().Faint(true)
	toolStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("13"))
	errorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	statusStyle  = lipgloss.NewStyle().Reverse(true).Padding(0, 1)
	inputHeight  = 3
	statusHeight = 1

	// eventSchema and sessionTable are the schema and table of the events of
	// sessions
	eventSchema  = "llm"
	sessionTable = "session"

	// memorySchema and detailTable are the schema and table of the events
	// of the memory's details
	memorySchema = "memory"
	detailTable  = "detail"

	// user is the label of the person's messages, when their name isn't
	// known, and assistant of the model's, when neither its name, nor its
	// provider, is
	user      = "You"
	assistant = "assistant"

	// indent is how far what's under a label is indented, which is as far
	// as markdown is
	indent = 2

	// watchRetry is how long after the changes can't be watched they're
	// watched again, which is twice as long each time, up to watchRetryMax,
	// and watchSettled is how long they're watched before they're taken to
	// be watched again
	watchRetry    = 2 * time.Second
	watchRetryMax = time.Minute
	watchSettled  = time.Second

	// thinking is shown for the model's thinking, unless debugging
	thinking = "Thinking…"

	// typingInterval is how often the server is told someone is typing, as
	// they type, which is within the server's TypingTimeout, of 10 seconds
	typingInterval = 5 * time.Second
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC FUNCTIONS

// Chat runs the interface to chat in the session with the identifier, until
// it's quit, with escape or control-c: its messages are shown, and what's
// typed is sent, with enter, and the model's reply is shown as it's
// generated, with that it's thinking, or, when debugging, its thinking,
// and the tools it calls, and their results. The changes which others make to the session, such as in
// another window, are shown as they're made, and whether someone is typing,
// or the model is replying, and the server is told when someone is typing.
func Chat(parent context.Context, client Client, id uuid.UUID, debug bool) error {
	// A reply which is being waited for is cancelled when the interface is
	// quit, so the server doesn't go on with it
	ctx, cancel := context.WithCancel(parent)
	defer cancel()

	// The changes to the session are watched before it's loaded, so none
	// are missed, and sent to the interface once it's running
	events := make(chan tea.Msg, 16)
	go watch(ctx, client, id, events)

	session, err := load(ctx, client, id)
	if err != nil {
		return err
	}
	input := textarea.New()
	input.Placeholder = "Ask a question, and press enter to send it, or escape to quit"
	input.ShowLineNumbers = false
	input.SetHeight(inputHeight)
	input.Focus()
	m := &model{ctx: ctx, client: client, debug: debug, session: session, history: viewport.New(80, 20), input: input}

	// The names of the person and the assistant, which aren't known, when
	// the server has no memory
	if details, err := client.GetDetails(ctx); err == nil {
		m.details = *details
	}
	program := tea.NewProgram(m, tea.WithAltScreen(), tea.WithContext(ctx))
	m.send = program.Send
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case msg := <-events:
				program.Send(msg)
			}
		}
	}()
	_, err = program.Run()
	return err
}

///////////////////////////////////////////////////////////////////////////////
// MODEL

func (m *model) Init() tea.Cmd {
	return textarea.Blink
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.input.SetWidth(msg.Width)
		m.history.Width = msg.Width
		m.history.Height = max(1, msg.Height-inputHeight-statusHeight-1)
		m.render()
		return m, nil
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEsc, tea.KeyCtrlC:
			return m, tea.Quit
		case tea.KeyEnter:
			return m, m.ask()
		case tea.KeyPgUp, tea.KeyPgDown:
			var cmd tea.Cmd
			m.history, cmd = m.history.Update(msg)
			return m, cmd
		}
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, tea.Batch(cmd, m.typing())
	case eventMsg:
		return m, m.changed(eventschema.Event(msg))
	case detailMsg:
		m.changes++
		m.detailChanged(eventschema.Event(msg))
		m.render()
		return m, nil
	case detailsMsg:
		// Details which changed as they were loaded may be older than the
		// changes, so they're loaded again
		if msg.err != nil {
			return m, nil
		} else if msg.changes != m.changes {
			return m, m.loadDetails()
		}
		m.details = *msg.details
		m.render()
		return m, nil
	case watchedMsg:
		// The changes aren't watched, such as when the server is stopping,
		// which is shown, until they're watched again, and the interface
		// carries on
		err := msg.err
		if err == nil || errors.Is(err, context.Canceled) {
			err = errors.New("the server stopped")
		}
		m.err = fmt.Errorf("%w, so changes aren't followed, until it's tried again in %v", err, msg.retry)
		m.render()
		return m, nil
	case rewatchedMsg:
		// The changes are watched again, so the session is loaded again, as
		// it may have changed since, and the error is cleared, unless
		// another has been shown
		if m.err != nil && strings.Contains(m.err.Error(), "changes aren't followed") {
			m.err = nil
		}
		m.render()
		return m, tea.Batch(m.load(), m.loadDetails())
	case loadedMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err
		} else if msg.session.Serial < m.session.Serial {
			// The session is older than the one shown, such as when a reply
			// was stored as it was loaded, so it's kept
		} else if !m.waiting {
			// The session is replaced, unless a reply is being waited for,
			// when it's loaded again once it's stored, keeping what tools
			// logged which is still where it was shown
			m.logs = keptLogs(m.logs, m.session.Messages, msg.session.Messages)
			m.session = msg.session
		} else {
			m.stale = true
		}
		m.render()
		if m.stale && !m.waiting {
			return m, m.load()
		}
		return m, nil
	case streamMsg:
		if msg.what == llm.StreamProgress {
			m.progress = progressText(llm.ParseProgress(msg.text))
		} else {
			m.streamed = addPiece(m.streamed, piece(msg))
		}
		m.render()
		return m, nil
	case replyMsg:
		// The question, as it was shown, is replaced by the messages, as
		// they were stored, or, when it wasn't sent, is typed again
		logged := m.streamed
		m.waiting, m.streamed, m.progress, m.err = false, nil, "", msg.err
		m.session.Messages = m.session.Messages[:msg.asked]
		var cmd tea.Cmd
		if msg.err != nil {
			// The session may have been changed elsewhere, such as when its
			// model replied to another window, so it's loaded again
			cmd = m.load()
			// The question is typed again, unless the next one has been
			// started, when it's shown with the error, so neither is lost
			if strings.TrimSpace(m.input.Value()) == "" {
				m.input.SetValue(msg.question)
			} else {
				m.err = fmt.Errorf("%w, so this wasn't sent: %s", msg.err, msg.question)
			}
		} else {
			// The reply was stored after changes made elsewhere, which
			// weren't shown, when its serial is more than one on, so the
			// session is loaded again
			skipped := msg.response.Serial > m.session.Serial+1
			m.keepLogs(logged, msg.asked, msg.response.Messages)
			m.session.Messages = append(m.session.Messages, msg.response.Messages...)
			m.session.Serial = msg.response.Serial
			m.session.Usage.Usage = addUsage(m.session.Usage.Usage, msg.response.Usage.Usage)
			m.session.Usage.Input = msg.response.Usage.Input
			m.session.Usage.Output = msg.response.Usage.Output
			m.session.Usage.Overhead = msg.response.Usage.Overhead
			if m.stale || skipped || m.seen > msg.response.Serial {
				cmd = m.load()
			}
		}
		m.render()
		return m, cmd
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m *model) View() string {
	return m.history.View() + "\n" + m.status() + "\n" + m.input.View()
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// changed returns the command which loads the session again, when the
// change is a new serial, which this window hasn't seen, such as a reply in
// another window, and shows its status, when it's changed, or that it's
// gone, when it's deleted
func (m *model) changed(event eventschema.Event) tea.Cmd {
	switch event.Action {
	case eventschema.EventDelete:
		m.err = errors.New("the session was deleted")
		m.render()
		return nil
	case eventschema.EventUpdate:
		var data schema.SessionEvent
		if err := json.Unmarshal(event.Data, &data); err != nil {
			return nil
		}
		if data.Status != "" {
			// A session which is being loaded is loaded again, as it may
			// have been got before the status changed
			m.session.Status = data.Status
			if m.loading {
				m.stale = true
			}
		}
		if data.Serial > m.session.Serial {
			// While a reply is waited for, the change may be its own, so
			// it's loaded once it's stored, when it isn't
			m.seen = max(m.seen, data.Serial)
			if !m.waiting {
				return m.load()
			}
		}
	}
	return nil
}

// detailChanged changes the detail of the event, which is forgotten, when
// it's deleted
func (m *model) detailChanged(event eventschema.Event) {
	field, err := memoryschema.ParseField(event.Row)
	if err != nil {
		return
	}
	var detail *memoryschema.Detail
	if event.Action != eventschema.EventDelete {
		detail = new(memoryschema.Detail)
		if err := json.Unmarshal(event.Data, detail); err != nil {
			return
		}
	}
	_ = m.details.Set(field, detail)
}

// loadDetails returns the command which loads the memory's details again,
// as they may have changed while the changes weren't watched
func (m *model) loadDetails() tea.Cmd {
	ctx, client, changes := m.ctx, m.client, m.changes
	return func() tea.Msg {
		details, err := client.GetDetails(ctx)
		return detailsMsg{details: details, err: err, changes: changes}
	}
}

// userLabel returns the label of the person's messages, which is their
// name, when it's known, or user
func (m *model) userLabel() string {
	if name, ok := m.details.Value(memoryschema.FieldName); ok {
		return name
	}
	return user
}

// assistantLabel returns the label of the model's messages, which is the
// assistant's name, when it's known, or the provider of the session's
// model, such as deepseek, for deepseek:deepseek-chat
func (m *model) assistantLabel() string {
	if name, ok := m.details.Value(memoryschema.FieldAssistant); ok {
		return name
	}
	if provider, _, ok := strings.Cut(types.Value(m.session.Model), ":"); ok && provider != "" {
		return provider
	}
	return assistant
}

// load returns the command which loads the session, with its messages,
// again, unless it's being loaded, when it's loaded again after
func (m *model) load() tea.Cmd {
	if m.loading {
		m.stale = true
		return nil
	}
	m.loading, m.stale = true, false
	ctx, client, id := m.ctx, m.client, m.session.ID
	return func() tea.Msg {
		session, err := load(ctx, client, id)
		return loadedMsg{session: session, err: err}
	}
}

// typing returns the command which tells the server someone is typing, when
// something is typed, and it wasn't told within typingInterval
func (m *model) typing() tea.Cmd {
	if strings.TrimSpace(m.input.Value()) == "" || time.Since(m.typed) < typingInterval {
		return nil
	}
	m.typed = time.Now()
	ctx, client, id := m.ctx, m.client, m.session.ID
	return func() tea.Msg {
		// It doesn't matter when it can't be told
		client.SetSessionStatus(ctx, id, schema.SessionTyping)
		return nil
	}
}

// ask sends what's typed, as a question, unless a reply is being waited for,
// and returns the command which waits for the reply, which is streamed
func (m *model) ask() tea.Cmd {
	text := strings.TrimSpace(m.input.Value())
	if text == "" || m.waiting {
		return nil
	}
	m.input.Reset()
	m.waiting, m.err, m.typed = true, nil, time.Time{}
	question := providerschema.MessageMeta{Sender: "user", Content: []providerschema.ContentBlock{{Text: types.Ptr(text)}}}
	m.session.Messages = append(m.session.Messages, providerschema.Message{MessageMeta: question})
	m.render()
	m.history.GotoBottom()

	// The question is shown as it's sent, and replaced by the session's
	// messages, as they were stored, once the reply is complete. The reply
	// is waited for, and streamed, apart from the model's state, which only
	// Update changes.
	ctx, client, id, send, asked := m.ctx, m.client, m.session.ID, m.send, len(m.session.Messages)-1
	return func() tea.Msg {
		response, err := client.Chat(ctx, id, schema.ChatRequest{Messages: []providerschema.MessageMeta{question}}, func(what llm.StreamType, text string) {
			send(streamMsg{what, text})
		})
		return replyMsg{response: response, err: err, asked: asked, question: text}
	}
}

// keepLogs keeps the lines which tools logged, as the reply was streamed,
// which aren't stored with the messages, so they're shown before the first
// of the tools' results, by its call's identifier, wherever it is when the
// session is loaded again, or, when it has none, or there are no results,
// by its position, or after the reply
func (m *model) keepLogs(streamed []*group, asked int, added []providerschema.Message) {
	var lines []string
	for _, g := range streamed {
		if g.what == llm.StreamLog {
			lines = append(lines, strings.TrimSpace(g.text.String()))
		}
	}
	if len(lines) == 0 {
		return
	}
	key := positionKey(asked + len(added))
	for i, message := range added {
		if message.Role() == "tool" {
			key = positionKey(asked + i)
			if results := message.ToolResults(); len(results) > 0 && results[0].ID != "" {
				key = resultKey(results[0].ID)
			}
			break
		}
	}
	if m.logs == nil {
		m.logs = make(map[string][]string)
	}
	m.logs[key] = append(m.logs[key], lines...)
}

// render renders the session's messages, and what's streamed of the reply,
// into the history, which is scrolled to the end
func (m *model) render() {
	var b strings.Builder
	m.speaker = ""
	for i, message := range m.session.Messages {
		m.renderLogs(&b, positionKey(i))
		if message.Role() == "tool" {
			for _, result := range message.ToolResults() {
				if result.ID != "" {
					m.renderLogs(&b, resultKey(result.ID))
				}
			}
		}
		m.renderMessage(&b, message)
	}
	m.renderLogs(&b, positionKey(len(m.session.Messages)))
	if len(m.streamed) > 0 {
		m.label(&b, "assistant")
		for _, g := range m.streamed {
			text := strings.Trim(g.text.String(), "\n")
			switch {
			case g.what == llm.StreamText:
				b.WriteString(m.body(lipgloss.NewStyle(), text) + "\n")
			case g.what == llm.StreamThinking && !m.debug:
				b.WriteString(m.body(dimStyle, thinking) + "\n")
			default:
				b.WriteString(m.body(dimStyle, text) + "\n")
			}
		}
		b.WriteString("\n")
	} else if m.waiting && m.progress == "" {
		b.WriteString(m.body(dimStyle, "Waiting for the reply…") + "\n")
	}
	if m.waiting && m.progress != "" {
		b.WriteString(m.body(dimStyle, m.progress) + "\n")
	}
	if m.err != nil {
		b.WriteString(errorStyle.Render("Error: "+m.err.Error()) + "\n")
	}
	// The history follows its end, as the reply is streamed, when it was
	// there, or it's the first time it's rendered, so it can be scrolled up
	// to read earlier messages as the reply is streamed
	follow := !m.rendered || m.history.AtBottom()
	m.history.SetContent(b.String())
	if follow {
		m.history.GotoBottom()
	}
	m.rendered = true
}

// renderLogs renders the lines which tools logged, which are kept by the key
// of what they're shown before
func (m *model) renderLogs(b *strings.Builder, key string) {
	// What tools log is the model's, so it's under its label, even when the
	// message which called them had nothing to show, but the tool calls
	if len(m.logs[key]) > 0 {
		m.label(b, "assistant")
	}
	for _, line := range m.logs[key] {
		b.WriteString(m.body(dimStyle, line) + "\n")
	}
}

// renderMessage renders a message: from the user, as it is, and from the
// model, its text, as markdown, and that it thought, or, when debugging,
// its thinking, dimmed, and the tools it called, and the results of tools,
// in a line each, by their canonical names. A message which has nothing to
// show isn't shown.
func (m *model) renderMessage(b *strings.Builder, message providerschema.Message) {
	switch message.Role() {
	case "user":
		m.label(b, "user")
		b.WriteString(m.body(lipgloss.NewStyle(), message.Text()) + "\n\n")
	case "assistant":
		var body strings.Builder
		thought := false
		for _, block := range message.Content {
			switch {
			case block.Thinking != nil && strings.TrimSpace(*block.Thinking) != "":
				if m.debug {
					body.WriteString(m.body(dimStyle, strings.TrimSpace(*block.Thinking)) + "\n")
				} else if !thought {
					body.WriteString(m.body(dimStyle, thinking) + "\n")
				}
				thought = true
			case block.Text != nil && strings.TrimSpace(*block.Text) != "":
				body.WriteString(m.markdown(*block.Text))
			case block.ToolCall != nil && m.debug:
				name := m.toolName(block.ToolCall.Name)
				body.WriteString(m.body(toolStyle, fmt.Sprintf("→ %s(%s)", name, oneLine(string(block.ToolCall.Input), m.width/2))) + "\n")
			}
		}
		if message.Result != "" && message.Result != providerschema.ResultStop && message.Result != providerschema.ResultToolCall {
			body.WriteString(m.body(errorStyle, "The reply stopped: "+message.Result.String()) + "\n")
		}
		if body.Len() > 0 {
			m.label(b, "assistant")
			b.WriteString(body.String() + "\n")
		}
	case "tool":
		if !m.debug {
			return
		}
		for _, result := range message.ToolResults() {
			name := m.toolName(result.Name)
			b.WriteString(m.body(dimStyle, fmt.Sprintf("← %s: %s", name, oneLine(string(result.Content), m.width-len(name)-6-indent))) + "\n")
		}
		b.WriteString("\n")
	}
}

// toolName returns the canonical name of the tool which a model calls by the
// name, such as homeassistant:call_service for homeassistant__call_service,
// or the name, when it isn't a tool in a namespace
func (m *model) toolName(name string) string {
	return toolschema.CanonicalFromModelToolName(name)
}

// wrap returns the text in the style, wrapped to fit the width, when it's
// known
func (m *model) wrap(style lipgloss.Style, text string) string {
	if m.width > 0 {
		style = style.Width(m.width)
	}
	return style.Render(text)
}

// label writes the label of the speaker, which is user, or assistant, unless
// they were the last labelled, so the model's messages in a turn, with what
// tools logged between them, are under one label
func (m *model) label(b *strings.Builder, speaker string) {
	if speaker == m.speaker {
		return
	}
	m.speaker = speaker
	if speaker == "user" {
		b.WriteString(youStyle.Render(m.userLabel()) + "\n")
	} else {
		b.WriteString(modelStyle.Render(m.assistantLabel()) + "\n")
	}
}

// body returns the text in the style, indented under its label, and
// wrapped to fit the width, when it's known
func (m *model) body(style lipgloss.Style, text string) string {
	return m.wrap(style.PaddingLeft(indent), text)
}

// markdown returns the text rendered as markdown, to fit the width, or as it
// is, when it can't be
func (m *model) markdown(text string) string {
	var b strings.Builder
	if m.width <= 0 {
		return text + "\n"
	} else if _, err := servertui.MarkdownFormatter(servertui.SetWidth(m.width)).Write(&b, text); err != nil {
		return text + "\n"
	}
	return b.String()
}

// status returns the status line: the session's model, identifier, and the
// tokens of its conversation, and its status, which is typing or replying,
// as the server says, in this window or another
func (m *model) status() string {
	parts := []string{types.Value(m.session.Model), m.session.ID.String()}
	if tokens := m.session.Usage.Input + m.session.Usage.Output + m.session.Usage.Overhead; tokens > 0 {
		parts = append(parts, fmt.Sprintf("%d tokens", tokens))
	}
	switch {
	case m.waiting || m.session.Status == schema.SessionReplying:
		parts = append(parts, "replying…")
	case m.session.Status == schema.SessionTyping:
		parts = append(parts, "typing…")
	}
	return statusStyle.Width(m.width).Render(strings.Join(parts, " · "))
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// watch sends the changes to the session with the identifier to the
// channel, until the context is cancelled. When the changes can't be
// watched, such as when the server is stopping, or can't be reached, that's
// sent, and they're watched again after watchRetry, and then twice as long
// each time, up to watchRetryMax. Once they've been watched for
// watchSettled, the first time too, the session is loaded again, as it may
// have changed before they were, and the wait starts again from watchRetry.
func watch(ctx context.Context, client Client, id uuid.UUID, events chan<- tea.Msg) {
	send := func(msg tea.Msg) bool {
		select {
		case events <- msg:
			return true
		case <-ctx.Done():
			return false
		}
	}
	retry := watchRetry
	for attempt := 0; ; attempt++ {
		// An attempt which lasts watchSettled is taken to be watching
		done := make(chan error, 1)
		go func() {
			done <- client.WatchEvents(ctx, func(event eventschema.Event) error {
				switch {
				case event.Schema == eventSchema && event.Table == sessionTable && event.Row == id.String():
					send(eventMsg(event))
				case event.Schema == memorySchema && event.Table == detailTable:
					send(detailMsg(event))
				}
				return nil
			})
		}()
		var err error
		select {
		case err = <-done:
		case <-time.After(watchSettled):
			// The session is loaded once the changes are watched, the first
			// time too, as it was loaded as they started to be, so a change
			// made between isn't missed
			if !send(rewatchedMsg{}) {
				return
			}
			retry = watchRetry
			err = <-done
		}
		if ctx.Err() != nil || !send(watchedMsg{err: err, retry: retry}) {
			return
		}

		// Wait, and then wait longer next time
		select {
		case <-ctx.Done():
			return
		case <-time.After(retry):
		}
		retry = min(retry*2, watchRetryMax)
	}
}

// keptLogs returns what tools logged, of the logs, which is kept when the
// earlier messages are replaced by the messages: what's kept by a tool's
// result, when the messages still have the result, wherever it is, such as
// after another window's turn, and what's kept by a position, when the
// messages carry on from the earlier ones, so not when they were forgotten,
// however many there are now. Returns nil when nothing is kept.
func keptLogs(logs map[string][]string, earlier, messages []providerschema.Message) map[string][]string {
	if len(logs) == 0 {
		return nil
	}
	results := make(map[string]bool)
	for _, message := range messages {
		for _, result := range message.ToolResults() {
			if result.ID != "" {
				results[resultKey(result.ID)] = true
			}
		}
	}
	carries := continues(earlier, messages)
	kept := make(map[string][]string, len(logs))
	for key, lines := range logs {
		if results[key] || (carries && strings.HasPrefix(key, "at:")) {
			kept[key] = lines
		}
	}
	if len(kept) == 0 {
		return nil
	}
	return kept
}

// continues returns whether the messages carry on from the earlier ones,
// which they start with, as they're the same conversation, with messages
// added to it
func continues(earlier, messages []providerschema.Message) bool {
	if len(messages) < len(earlier) {
		return false
	}
	for i := range earlier {
		a, errA := json.Marshal(earlier[i])
		b, errB := json.Marshal(messages[i])
		if errA != nil || errB != nil || !bytes.Equal(a, b) {
			return false
		}
	}
	return true
}

// resultKey and positionKey return the keys of the lines which tools
// logged, which are shown before the result of the call with the
// identifier, or the message at the position
func resultKey(id string) string {
	return "result:" + id
}

func positionKey(i int) string {
	return "at:" + strconv.Itoa(i)
}

// load returns the session with the identifier, with its messages
func load(ctx context.Context, client Client, id uuid.UUID) (*schema.Session, error) {
	session, err := client.GetSession(ctx, id)
	if err != nil {
		return nil, err
	}
	messages, err := client.ListSessionMessages(ctx, id, schema.MessageListRequest{})
	if err != nil {
		return nil, err
	}
	session.Messages = messages.Body
	return session, nil
}

// addPiece adds the piece of a reply, as it's streamed, to the groups:
// to the last, when it's of the same kind, apart from a tool's log line,
// which is a group of its own, or to a new group, when it isn't
func addPiece(groups []*group, p piece) []*group {
	if n := len(groups); n > 0 && groups[n-1].what == p.what && p.what != llm.StreamLog {
		groups[n-1].text.WriteString(p.text)
		return groups
	}
	g := &group{what: p.what}
	g.text.WriteString(p.text)
	return append(groups, g)
}

// progressText returns a tool's progress, as it's shown: its message, and
// its percent, when it's known
func progressText(p llm.Progress) string {
	if percent, ok := p.Percent(); ok {
		return fmt.Sprintf("%s (%.0f%%)", p.Message, percent)
	}
	return p.Message
}

// oneLine returns the text on one line, of at most the characters
func oneLine(text string, chars int) string {
	text = strings.Join(strings.Fields(text), " ")
	if chars <= 1 {
		chars = 40
	}
	if runes := []rune(text); len(runes) > chars {
		return string(runes[:chars-1]) + "…"
	}
	return text
}

// addUsage returns the tokens, pages and seconds of both usages
func addUsage(a, b llm.Usage) llm.Usage {
	return a.Add(b)
}
