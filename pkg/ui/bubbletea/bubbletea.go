// Package bubbletea implements ui.ChatUI for interactive terminals using
// the Charm bubbletea framework. It provides a scrollable chat history,
// a text input prompt, a typing/spinner indicator, and Markdown rendering
// via glamour.
package bubbletea

import (
	"context"
	"fmt"
	"io"
	"os/user"
	"strings"
	"sync"

	// Packages
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/reflow/wordwrap"
	"github.com/muesli/termenv"
	"github.com/mutablelogic/go-llm/pkg/ui"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Terminal implements ui.ChatUI for interactive terminal sessions.
type Terminal struct {
	program *tea.Program
	events  chan ui.Event // incoming events from the TUI to the caller
	done    chan struct{} // closed when the program exits
	err     error         // error from program.Run
	mu      sync.Mutex
	model   *model
}

// model is the bubbletea model that manages the TUI state.
type model struct {
	viewport  viewport.Model
	input     textinput.Model
	spinner   spinner.Model
	history   []historyEntry
	typing    bool
	width     int
	height    int
	ready     bool
	events    chan<- ui.Event
	renderer  *glamour.TermRenderer
	stylePath string // glamour style ("dark" or "light"), detected before TUI starts
	ctx       *termContext
	quitting  bool
}

type historyEntry struct {
	role      string        // "user", "assistant", "system", "error"
	text      string        // rendered text
	rawText   string        // raw text before rendering (used for streaming)
	segments  []textSegment // role-tagged segments accumulated during streaming
	streaming bool          // true while this entry is being streamed
	glamoured bool          // true if text was rendered through glamour
}

// textSegment is a chunk of streamed text tagged with its source role.
type textSegment struct {
	role string // "assistant", "thinking", "tool"
	text string
}

// termContext implements ui.Context for terminal sessions.
type termContext struct {
	program  *tea.Program
	userID   string
	userName string
}

///////////////////////////////////////////////////////////////////////////////
// MESSAGES (bubbletea internal)

type appendMsg struct {
	role string
	text string
}

type typingMsg struct {
	typing bool
}

type streamStartMsg struct{}

type streamChunkMsg struct {
	role string
	text string
}

type streamEndMsg struct{}

type clearMsg struct{}

///////////////////////////////////////////////////////////////////////////////
// STYLES

var (
	userStyle      = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12")) // blue
	assistantStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("10")) // green
	systemStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("11")) // yellow
	errorStyle     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("9"))  // red
	dimStyle       = lipgloss.NewStyle().Faint(true)
	promptStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("14")) // cyan
)

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// New creates a new terminal chat UI. The UI takes over the terminal
// and should be closed when done.
func New() (*Terminal, error) {
	// Resolve current user
	username := "user"
	uid := "terminal"
	if u, err := user.Current(); err == nil {
		username = u.Username
		uid = u.Uid
	}

	// Detect terminal background BEFORE starting bubbletea, so that
	// the escape-sequence response is consumed here rather than leaking
	// into bubbletea's input reader.
	stylePath := "dark"
	if !termenv.HasDarkBackground() {
		stylePath = "light"
	}

	events := make(chan ui.Event, 1)

	tctx := &termContext{
		userID:   uid,
		userName: username,
	}

	// Create bubbles components
	ti := textinput.New()
	ti.Placeholder = "Type a message..."
	ti.Focus()
	ti.CharLimit = 0 // unlimited

	sp := spinner.New(spinner.WithSpinner(spinner.Dot))

	m := &model{
		input:     ti,
		spinner:   sp,
		events:    events,
		stylePath: stylePath,
		ctx:       tctx,
	}

	t := &Terminal{
		events: events,
		done:   make(chan struct{}),
		model:  m,
	}

	p := tea.NewProgram(m, tea.WithAltScreen())
	t.program = p
	tctx.program = p

	// Run the TUI in a background goroutine
	go func() {
		defer close(t.done)
		if _, err := p.Run(); err != nil {
			t.mu.Lock()
			t.err = err
			t.mu.Unlock()
		}
		close(events)
	}()

	return t, nil
}

///////////////////////////////////////////////////////////////////////////////
// ui.ChatUI IMPLEMENTATION

// Receive blocks until the next user event or until the context is cancelled.
func (t *Terminal) Receive(ctx context.Context) (ui.Event, error) {
	select {
	case <-ctx.Done():
		return ui.Event{}, ctx.Err()
	case evt, ok := <-t.events:
		if !ok {
			t.mu.Lock()
			err := t.err
			t.mu.Unlock()
			if err != nil {
				return ui.Event{}, err
			}
			return ui.Event{}, io.EOF
		}
		return evt, nil
	}
}

// Close shuts down the terminal UI.
func (t *Terminal) Close() error {
	t.program.Quit()
	<-t.done
	return nil
}

// AppendHistory adds a message to the chat display without generating
// an event. Used to restore previous conversation history.
func (t *Terminal) AppendHistory(role, text string) {
	t.program.Send(appendMsg{role: role, text: text})
}

// ClearHistory clears all messages from the chat display.
func (t *Terminal) ClearHistory() {
	t.program.Send(clearMsg{})
}

// ClearHistory clears the chat display.
func (c *termContext) ClearHistory() {
	c.program.Send(clearMsg{})
}

// AppendHistory adds a pre-existing message to the chat display.
func (c *termContext) AppendHistory(role, text string) {
	c.program.Send(appendMsg{role: role, text: text})
}

///////////////////////////////////////////////////////////////////////////////
// ui.Context IMPLEMENTATION

func (c *termContext) UserID() string         { return c.userID }
func (c *termContext) UserName() string       { return c.userName }
func (c *termContext) ConversationID() string { return "terminal" }

func (c *termContext) SendText(ctx context.Context, text string) error {
	c.program.Send(appendMsg{role: "system", text: text})
	return nil
}

func (c *termContext) SendMarkdown(ctx context.Context, markdown string) error {
	c.program.Send(appendMsg{role: "assistant", text: markdown})
	return nil
}

func (c *termContext) SendAttachment(ctx context.Context, att ui.OutAttachment) error {
	c.program.Send(appendMsg{
		role: "system",
		text: fmt.Sprintf("[Attachment: %s (%s)]", att.Filename, att.Type),
	})
	return nil
}

func (c *termContext) SetTyping(ctx context.Context, typing bool) error {
	c.program.Send(typingMsg{typing: typing})
	return nil
}

func (c *termContext) StreamStart(ctx context.Context) error {
	c.program.Send(streamStartMsg{})
	return nil
}

func (c *termContext) StreamChunk(ctx context.Context, role, text string) error {
	c.program.Send(streamChunkMsg{role: role, text: text})
	return nil
}

func (c *termContext) StreamEnd(ctx context.Context) error {
	c.program.Send(streamEndMsg{})
	return nil
}

///////////////////////////////////////////////////////////////////////////////
// BUBBLETEA MODEL

func (m *model) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, m.spinner.Tick)
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			m.quitting = true
			return m, tea.Quit
		case tea.KeyEnter:
			text := strings.TrimSpace(m.input.Value())
			if text == "" {
				return m, nil
			}
			m.input.SetValue("")

			// Add user message to history
			m.history = append(m.history, historyEntry{role: "user", text: text})
			m.updateViewport()

			// Parse command or text event
			evt := m.parseEvent(text)
			m.events <- evt

			return m, nil
		}

	case tea.WindowSizeMsg:
		footerHeight := 2 // input + status line
		m.width = msg.Width
		m.height = msg.Height

		if !m.ready {
			m.viewport = viewport.New(msg.Width, msg.Height-footerHeight)
			m.viewport.YPosition = 0
			m.ready = true
		} else {
			m.viewport.Width = msg.Width
			m.viewport.Height = msg.Height - footerHeight
		}
		m.input.Width = msg.Width - 4

		// (Re)create glamour renderer with new width and re-render history
		m.newRenderer()
		m.rerenderHistory()
		m.updateViewport()
		return m, nil

	case appendMsg:
		rendered := msg.text
		raw := msg.text
		glamoured := false
		// Try to render markdown for assistant and system messages
		if (msg.role == "assistant" || msg.role == "system") && m.renderer != nil {
			if out, err := m.renderer.Render(msg.text); err == nil {
				rendered = trimGlamour(out)
				glamoured = true
			}
		}
		m.history = append(m.history, historyEntry{role: msg.role, text: rendered, rawText: raw, glamoured: glamoured})
		m.typing = false
		m.updateViewport()
		return m, nil

	case streamStartMsg:
		m.history = append(m.history, historyEntry{streaming: true})
		m.typing = true
		m.updateViewport()
		return m, nil

	case streamChunkMsg:
		if n := len(m.history); n > 0 && m.history[n-1].streaming {
			entry := &m.history[n-1]
			entry.rawText += msg.text

			// Set the entry's role from the first chunk received
			if entry.role == "" {
				entry.role = msg.role
			}

			// Append to the current segment or start a new one when role changes
			if len(entry.segments) > 0 && entry.segments[len(entry.segments)-1].role == msg.role {
				entry.segments[len(entry.segments)-1].text += msg.text
			} else {
				entry.segments = append(entry.segments, textSegment{role: msg.role, text: msg.text})
			}

			// Render segments with role-based styling for streaming display
			entry.text = m.renderSegments(entry.segments)
			m.updateViewport()
		}
		return m, nil

	case streamEndMsg:
		if n := len(m.history); n > 0 && m.history[n-1].streaming {
			entry := &m.history[n-1]
			entry.streaming = false
			// Default role to "assistant" if no chunks were received
			if entry.role == "" {
				entry.role = "assistant"
			}

			if len(entry.segments) <= 1 {
				// Single segment: render in place with glamour
				entry.text = m.renderSegmentsFinal(entry.segments)
			} else {
				// Multiple segments: split into separate history entries
				// so each gets its own top-level role label.
				segs := entry.segments
				entry.role = segs[0].role
				entry.text = m.renderSingleSegment(segs[0])
				entry.segments = nil

				for _, seg := range segs[1:] {
					m.history = append(m.history, historyEntry{
						role:    seg.role,
						text:    m.renderSingleSegment(seg),
						rawText: seg.text,
					})
				}
			}
		}
		m.typing = false
		m.updateViewport()
		return m, nil

	case typingMsg:
		m.typing = msg.typing
		return m, nil

	case clearMsg:
		m.history = nil
		m.updateViewport()
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)
	}

	// Update text input
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	cmds = append(cmds, cmd)

	// Forward navigation keys to viewport for scrolling, but block regular
	// typing keys to prevent the viewport jumping on each keystroke.
	if keyMsg, isKey := msg.(tea.KeyMsg); isKey {
		switch keyMsg.Type {
		case tea.KeyUp, tea.KeyDown, tea.KeyPgUp, tea.KeyPgDown, tea.KeyHome, tea.KeyEnd:
			m.viewport, cmd = m.viewport.Update(msg)
			cmds = append(cmds, cmd)
		}
	} else {
		m.viewport, cmd = m.viewport.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m *model) View() string {
	if m.quitting {
		return ""
	}
	if !m.ready {
		return "\n  Initializing..."
	}

	// Status line
	var status string
	if m.typing {
		status = dimStyle.Render(m.spinner.View() + " thinking...")
	} else {
		status = dimStyle.Render("ctrl+c to quit")
	}

	// Input
	input := m.input.View()

	return fmt.Sprintf("%s\n%s\n%s", m.viewport.View(), input, status)
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// trimGlamour trims leading/trailing blank lines from glamour output.
func trimGlamour(s string) string {
	return strings.TrimSpace(s)
}

// indentText ensures every line has a 2-space indent, matching glamour's
// default left margin so all content is visually consistent.
func indentText(s string) string {
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		if line == "" {
			continue
		}
		// Only add indent if the line doesn't already start with spaces
		if len(line) < 2 || line[:2] != "  " {
			lines[i] = "  " + line
		}
	}
	return strings.Join(lines, "\n")
}

// renderSegments produces styled, word-wrapped text from role-tagged
// segments. Assistant text is plain, thinking text is dim/faint, and
// tool text is yellow. When the role changes between segments, a role
// label is printed for the new segment.
func (m *model) renderSegments(segs []textSegment) string {
	w := m.wrapWidth()
	var b strings.Builder
	for i, seg := range segs {
		if i > 0 {
			// Insert separation and a role label when switching roles
			b.WriteString("\n\n" + m.styleRole(seg.role) + "\n")
		}
		wrapped := wordwrap.String(seg.text, w)
		switch seg.role {
		case "thinking":
			b.WriteString(dimStyle.Render(wrapped))
		case "tool":
			b.WriteString(dimStyle.Render(wrapped))
		default:
			b.WriteString(wrapped)
		}
	}
	return b.String()
}

// renderSegmentsFinal renders each segment with full glamour markdown
// formatting, with role labels between segments when the role changes.
func (m *model) renderSegmentsFinal(segs []textSegment) string {
	var b strings.Builder
	for i, seg := range segs {
		if i > 0 {
			b.WriteString("\n\n" + m.styleRole(seg.role) + "\n")
		}
		b.WriteString(m.renderSingleSegment(seg))
	}
	return b.String()
}

// renderSingleSegment renders a single text segment with glamour markdown
// and applies role-based styling (dim for thinking/tool).
func (m *model) renderSingleSegment(seg textSegment) string {
	var rendered string
	if m.renderer != nil {
		if out, err := m.renderer.Render(seg.text); err == nil {
			rendered = trimGlamour(out)
		} else {
			rendered = wordwrap.String(seg.text, m.wrapWidth())
		}
	} else {
		rendered = wordwrap.String(seg.text, m.wrapWidth())
	}
	switch seg.role {
	case "thinking", "tool":
		return dimStyle.Render(rendered)
	default:
		return rendered
	}
}

// wrapWidth returns the available text width for content, accounting for
// the role prefix and some padding.
func (m *model) wrapWidth() int {
	const margin = 14 // "assistant: " + padding
	w := max(m.width-margin, 20)
	return w
}

// newRenderer creates a glamour terminal renderer with the given wrap width.
// Uses the pre-detected style path to avoid querying the terminal inside
// bubbletea's event loop.
func (m *model) newRenderer() {
	r, err := glamour.NewTermRenderer(
		glamour.WithStylePath(m.stylePath),
		glamour.WithWordWrap(m.wrapWidth()),
	)
	if err == nil {
		m.renderer = r
	}
}

// rerenderHistory re-renders all glamour-rendered history entries with the
// current renderer (e.g. after a terminal resize).
func (m *model) rerenderHistory() {
	if m.renderer == nil {
		return
	}
	for i := range m.history {
		if m.history[i].streaming {
			continue
		}
		if m.history[i].role != "assistant" && m.history[i].role != "system" {
			continue
		}
		raw := m.history[i].rawText
		if raw == "" {
			continue
		}
		if out, err := m.renderer.Render(raw); err == nil {
			m.history[i].text = trimGlamour(out)
			m.history[i].glamoured = true
		}
	}
}

func (m *model) updateViewport() {
	var b strings.Builder
	for _, entry := range m.history {
		if entry.role != "" {
			b.WriteString(m.styleRole(entry.role))
		}
		if entry.text != "" {
			if entry.glamoured {
				// Glamour already handles margins and wrapping
				b.WriteString("\n" + entry.text)
			} else {
				b.WriteString("\n" + indentText(entry.text))
			}
		}
		b.WriteString("\n\n")
	}
	m.viewport.SetContent(b.String())
	m.viewport.GotoBottom()
}

func (m *model) styleRole(role string) string {
	switch role {
	case "user":
		return userStyle.Render(role + ":")
	case "assistant":
		return assistantStyle.Render(role + ":")
	case "thinking":
		return dimStyle.Render(role + ":")
	case "tool":
		return dimStyle.Render(role + ":")
	case "system":
		return systemStyle.Render(role + ":")
	case "error":
		return errorStyle.Render(role + ":")
	default:
		return role + ":"
	}
}

func (m *model) parseEvent(text string) ui.Event {
	evt := ui.Event{
		Context: m.ctx,
	}

	if strings.HasPrefix(text, "/") {
		parts := strings.Fields(text)
		evt.Type = ui.EventCommand
		evt.Text = text
		evt.Command = strings.TrimPrefix(parts[0], "/")
		if len(parts) > 1 {
			evt.Args = parts[1:]
		}
	} else {
		evt.Type = ui.EventText
		evt.Text = text
	}

	return evt
}
