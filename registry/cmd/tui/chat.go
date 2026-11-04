// Package tui is a terminal interface to chat in a session, which is a
// throwaway, until there's a better one
package tui

import (
	"context"
	"fmt"
	"strings"
	"uuid"

	// Packages
	textarea "github.com/charmbracelet/bubbles/textarea"
	viewport "github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	lipgloss "github.com/charmbracelet/lipgloss"
	llm "github.com/mutablelogic/go-llm"
	providerschema "github.com/mutablelogic/go-llm/provider/schema"
	schema "github.com/mutablelogic/go-llm/registry/schema"
	servertui "github.com/mutablelogic/go-server/pkg/tui"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Client is what the interface chats in a session with
type Client interface {
	GetSession(ctx context.Context, id uuid.UUID) (*schema.Session, error)
	ListSessionMessages(ctx context.Context, id uuid.UUID, req schema.MessageListRequest) (*schema.MessageList, error)
	Chat(ctx context.Context, id uuid.UUID, req schema.ChatRequest, stream llm.GeneratorStreamFn) (*schema.ChatResponse, error)
}

// model is the interface's state: the session, its messages, what's being
// streamed of the reply, and what's typed
type model struct {
	ctx      context.Context
	client   Client
	session  *schema.Session
	send     func(tea.Msg)
	history  viewport.Model
	input    textarea.Model
	width    int
	waiting  bool
	streamed []*group
	rendered bool
	err      error
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
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC FUNCTIONS

// Chat runs the interface to chat in the session with the identifier, until
// it's quit, with escape or control-c: its messages are shown, and what's
// typed is sent, with enter, and the model's reply is shown as it's
// generated
func Chat(parent context.Context, client Client, id uuid.UUID) error {
	// A reply which is being waited for is cancelled when the interface is
	// quit, so the server doesn't go on with it
	ctx, cancel := context.WithCancel(parent)
	defer cancel()

	session, err := client.GetSession(ctx, id)
	if err != nil {
		return err
	}
	messages, err := client.ListSessionMessages(ctx, id, schema.MessageListRequest{})
	if err != nil {
		return err
	}
	session.Messages = messages.Body
	input := textarea.New()
	input.Placeholder = "Ask a question, and press enter to send it, or escape to quit"
	input.ShowLineNumbers = false
	input.SetHeight(inputHeight)
	input.Focus()
	m := &model{ctx: ctx, client: client, session: session, history: viewport.New(80, 20), input: input}
	program := tea.NewProgram(m, tea.WithAltScreen(), tea.WithContext(ctx))
	m.send = program.Send
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
	case streamMsg:
		m.streamed = addPiece(m.streamed, piece(msg))
		m.render()
		return m, nil
	case replyMsg:
		// The question, as it was shown, is replaced by the messages, as
		// they were stored, or, when it wasn't sent, is typed again
		m.waiting, m.streamed, m.err = false, nil, msg.err
		m.session.Messages = m.session.Messages[:msg.asked]
		if msg.err != nil {
			// The question is typed again, unless the next one has been
			// started, when it's shown with the error, so neither is lost
			if strings.TrimSpace(m.input.Value()) == "" {
				m.input.SetValue(msg.question)
			} else {
				m.err = fmt.Errorf("%w, so this wasn't sent: %s", msg.err, msg.question)
			}
		} else {
			m.session.Messages = append(m.session.Messages, msg.response.Messages...)
			m.session.Serial = msg.response.Serial
			m.session.Usage.Usage = addUsage(m.session.Usage.Usage, msg.response.Usage.Usage)
			m.session.Usage.Input = msg.response.Usage.Input
			m.session.Usage.Output = msg.response.Usage.Output
			m.session.Usage.Overhead = msg.response.Usage.Overhead
		}
		m.render()
		return m, nil
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

// ask sends what's typed, as a question, unless a reply is being waited for,
// and returns the command which waits for the reply, which is streamed
func (m *model) ask() tea.Cmd {
	text := strings.TrimSpace(m.input.Value())
	if text == "" || m.waiting {
		return nil
	}
	m.input.Reset()
	m.waiting, m.err = true, nil
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

// render renders the session's messages, and what's streamed of the reply,
// into the history, which is scrolled to the end
func (m *model) render() {
	var b strings.Builder
	for _, message := range m.session.Messages {
		m.renderMessage(&b, message)
	}
	if len(m.streamed) > 0 {
		b.WriteString(modelStyle.Render("Model") + "\n")
		for _, g := range m.streamed {
			text := strings.Trim(g.text.String(), "\n")
			switch g.what {
			case llm.StreamText:
				b.WriteString(m.wrap(lipgloss.NewStyle(), text) + "\n")
			default:
				b.WriteString(m.wrap(dimStyle, text) + "\n")
			}
		}
		b.WriteString("\n")
	} else if m.waiting {
		b.WriteString(dimStyle.Render("Waiting for the reply…") + "\n")
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

// renderMessage renders a message: from the user, as it is, from the model,
// its thinking, dimmed, its text, as markdown, and the tools it called, and
// the results of tools, in a line each
func (m *model) renderMessage(b *strings.Builder, message providerschema.Message) {
	switch message.Role() {
	case "user":
		b.WriteString(youStyle.Render("You") + "\n" + m.wrap(lipgloss.NewStyle(), message.Text()) + "\n\n")
	case "assistant":
		b.WriteString(modelStyle.Render("Model") + "\n")
		for _, block := range message.Content {
			switch {
			case block.Thinking != nil && strings.TrimSpace(*block.Thinking) != "":
				b.WriteString(m.wrap(dimStyle, strings.TrimSpace(*block.Thinking)) + "\n")
			case block.Text != nil && strings.TrimSpace(*block.Text) != "":
				b.WriteString(m.markdown(*block.Text))
			case block.ToolCall != nil:
				b.WriteString(toolStyle.Render(fmt.Sprintf("→ %s(%s)", block.ToolCall.Name, oneLine(string(block.ToolCall.Input), m.width/2))) + "\n")
			}
		}
		if message.Result != "" && message.Result != providerschema.ResultStop && message.Result != providerschema.ResultToolCall {
			b.WriteString(errorStyle.Render("The reply stopped: "+message.Result.String()) + "\n")
		}
		b.WriteString("\n")
	case "tool":
		for _, result := range message.ToolResults() {
			b.WriteString(dimStyle.Render(fmt.Sprintf("← %s: %s", result.Name, oneLine(string(result.Content), m.width-len(result.Name)-6))) + "\n")
		}
		b.WriteString("\n")
	}
}

// wrap returns the text in the style, wrapped to fit the width, when it's
// known
func (m *model) wrap(style lipgloss.Style, text string) string {
	if m.width > 0 {
		style = style.Width(m.width)
	}
	return style.Render(text)
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
// tokens of its conversation, and whether a reply is being waited for
func (m *model) status() string {
	parts := []string{types.Value(m.session.Model), m.session.ID.String()}
	if tokens := m.session.Usage.Input + m.session.Usage.Output + m.session.Usage.Overhead; tokens > 0 {
		parts = append(parts, fmt.Sprintf("%d tokens", tokens))
	}
	if m.waiting {
		parts = append(parts, "replying…")
	}
	return statusStyle.Width(m.width).Render(strings.Join(parts, " · "))
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

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

// addUsage returns the tokens, and pages, of both usages
func addUsage(a, b llm.Usage) llm.Usage {
	return llm.Usage{
		InputTokens:     a.InputTokens + b.InputTokens,
		OutputTokens:    a.OutputTokens + b.OutputTokens,
		CachedTokens:    a.CachedTokens + b.CachedTokens,
		ReasoningTokens: a.ReasoningTokens + b.ReasoningTokens,
		Pages:           a.Pages + b.Pages,
		Seconds:         a.Seconds + b.Seconds,
	}
}
