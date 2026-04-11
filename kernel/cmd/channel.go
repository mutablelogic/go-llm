package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	// Packages
	tea "github.com/charmbracelet/bubbletea"
	uuid "github.com/google/uuid"
	goclient "github.com/mutablelogic/go-client"
	otel "github.com/mutablelogic/go-client/pkg/otel"
	httpclient "github.com/mutablelogic/go-llm/kernel/httpclient"
	schema "github.com/mutablelogic/go-llm/kernel/schema"
	tui "github.com/mutablelogic/go-llm/pkg/tui"
	server "github.com/mutablelogic/go-server"
	httpresponse "github.com/mutablelogic/go-server/pkg/httpresponse"
	attribute "go.opentelemetry.io/otel/attribute"
	term "golang.org/x/term"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type ChannelCommands struct {
	Channel ChannelCommand `cmd:"" name:"channel" help:"Open the interactive session channel debug endpoint." group:"RESPONSES"`
}

type ChannelCommand struct {
	Session uuid.UUID `name:"session" help:"Session ID (defaults to the stored current session)." optional:""`
}

type channelFrameMsg struct {
	title string
	frame json.RawMessage
}

type channelErrMsg struct {
	err error
}

type channelBlinkMsg struct{}

type channelModel struct {
	viewport      *tui.Viewport
	input         string
	status        string
	streaming     bool
	spinnerFrame  int
	activeRole    string
	quitting      bool
	send          func(string) error
	turn          int
	live          map[string]struct{}
	promptVisible bool
}

type channelResponseSection struct {
	role     string
	markdown string
}

const (
	channelBlinkInterval = 300 * time.Millisecond
	channelCursorGlyph   = "█"
)

var channelSpinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

func (cmd *ChannelCommand) Run(ctx server.Cmd) (err error) {
	if !term.IsTerminal(int(os.Stdin.Fd())) || !term.IsTerminal(int(os.Stdout.Fd())) {
		return fmt.Errorf("channel requires an interactive terminal")
	}

	id, err := resolveSessionID(cmd.Session, ctx.GetString("session"))
	if err != nil {
		return err
	}
	if err := ctx.Set("session", id.String()); err != nil {
		return err
	}

	return WithClient(ctx, func(client *httpclient.Client, _ string) error {
		parent, endSpan := otel.StartSpan(ctx.Tracer(), ctx.Context(), "ChannelCommand",
			attribute.String("session", id.String()),
		)
		defer func() { endSpan(err) }()

		return client.Channel(parent, id, func(streamctx context.Context, stream goclient.JSONStream) error {
			uictx, cancel := context.WithCancel(streamctx)
			defer cancel()

			model := newChannelModel(stream)
			program := tea.NewProgram(model, tea.WithAltScreen())

			recvErrCh := make(chan error, 1)
			go func() {
				recvErrCh <- pumpChannelFrames(uictx, program, stream)
			}()

			_, runErr := program.Run()
			cancel()
			recvErr := <-recvErrCh

			if runErr != nil {
				return runErr
			}
			if recvErr != nil && !errors.Is(recvErr, context.Canceled) && !errors.Is(recvErr, io.EOF) {
				return recvErr
			}

			return nil
		})
	})
}

func newChannelModel(stream goclient.JSONStream) *channelModel {
	width, height := 80, 24
	if w, h, err := term.GetSize(int(os.Stdout.Fd())); err == nil {
		width, height = w, h
	}

	viewport := tui.NewViewport(tui.SetWidth(width), tui.SetHeight(max(1, height-2)))
	return &channelModel{
		viewport:      viewport,
		status:        "enter to send, pgup/pgdn to scroll, ctrl+c to quit",
		live:          make(map[string]struct{}),
		promptVisible: true,
		send: func(text string) error {
			payload, err := channelPayload(text)
			if err != nil {
				return err
			}
			return stream.Send(payload)
		},
	}
}

func (m *channelModel) Init() tea.Cmd {
	m.promptVisible = true
	return channelBlinkCmd()
}

func (m *channelModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		_ = m.viewport.Resize(msg.Width, max(1, msg.Height-2))
		return m, nil
	case channelFrameMsg:
		if err := m.applyFrame(msg.title, msg.frame); err != nil {
			m.status = err.Error()
			return m, tea.Quit
		}
		return m, nil
	case channelErrMsg:
		_ = m.clearCursor()
		m.status = msg.err.Error()
		return m, tea.Quit
	case channelBlinkMsg:
		if err := m.tickCursor(); err != nil {
			m.status = err.Error()
			return m, tea.Quit
		}
		return m, channelBlinkCmd()
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			m.quitting = true
			return m, tea.Quit
		case tea.KeyEnter:
			text := strings.TrimSpace(m.input)
			if text == "" {
				return m, nil
			}
			if err := m.send(text); err != nil {
				m.status = err.Error()
				return m, tea.Quit
			}
			if err := m.viewport.Append(channelRequestMarkdown(text)); err != nil {
				m.status = err.Error()
				return m, tea.Quit
			}
			if err := m.startStreaming(); err != nil {
				m.status = err.Error()
				return m, tea.Quit
			}
			m.turn++
			m.live = make(map[string]struct{})
			m.input = ""
			m.status = "sent"
			return m, nil
		case tea.KeyBackspace, tea.KeyDelete:
			m.input = trimLastRune(m.input)
			return m, nil
		case tea.KeyUp:
			m.viewport.LineUp(1)
			return m, nil
		case tea.KeyDown:
			m.viewport.LineDown(1)
			return m, nil
		case tea.KeyPgUp:
			m.viewport.PageUp()
			return m, nil
		case tea.KeyPgDown:
			m.viewport.PageDown()
			return m, nil
		case tea.KeyHome:
			m.viewport.GotoTop()
			return m, nil
		case tea.KeyEnd:
			m.viewport.GotoBottom()
			return m, nil
		case tea.KeyCtrlU:
			m.input = ""
			return m, nil
		case tea.KeySpace:
			m.input += " "
			return m, nil
		default:
			if len(msg.Runes) > 0 {
				m.input += string(msg.Runes)
			}
			return m, nil
		}
	}

	return m, nil
}

func (m *channelModel) View() string {
	if m.quitting {
		return ""
	}

	content := m.viewport.View()
	if content == "" {
		content = "Connecting..."
	}
	prompt := m.input
	if m.promptVisible {
		prompt += channelCursorGlyph
	}
	status := m.status
	if m.streaming {
		status = m.spinnerGlyph() + " " + m.streamingLabel()
	}

	return fmt.Sprintf("%s\n> %s\n%s", content, prompt, status)
}

func (m *channelModel) applyFrame(title string, frame json.RawMessage) error {
	if delta, ok := decodeChannelDeltaFrame(frame); ok {
		return m.applyDelta(delta)
	}
	if response, ok := decodeChannelResponseFrame(frame); ok {
		return m.applyResponse(response)
	}
	if errFrame, ok := decodeChannelErrorFrame(frame); ok {
		if err := m.viewport.Append(channelErrorMarkdown(errFrame)); err != nil {
			return err
		}
		if err := m.clearCursor(); err != nil {
			return err
		}
		m.status = errFrame.Reason
		return nil
	}

	markdown, err := channelFrameMarkdown(title, frame)
	if err != nil {
		return err
	}
	if err := m.viewport.Append(markdown); err != nil {
		return err
	}
	if title == "session" {
		m.status = "connected"
	} else {
		m.status = "received"
	}
	return nil
}

func (m *channelModel) applyDelta(delta schema.StreamDelta) error {
	if m.turn == 0 {
		m.turn = 1
	}
	key := m.sectionKey(delta.Role)
	if _, ok := m.live[key]; ok {
		if err := m.viewport.AppendSection(key, delta.Text); err != nil {
			return err
		}
	} else {
		if err := m.viewport.SetSection(key, channelDeltaMarkdown(delta.Role, delta.Text)); err != nil {
			return err
		}
		m.live[key] = struct{}{}
	}
	m.status = fmt.Sprintf("streaming %s", delta.Role)
	m.activeRole = delta.Role
	m.streaming = true
	return nil
}

func (m *channelModel) applyResponse(response schema.ChatResponse) error {
	sections, err := channelResponseSections(response)
	if err != nil {
		return err
	}
	if len(sections) == 0 {
		return nil
	}
	current := make(map[string]struct{}, len(sections))
	for _, section := range sections {
		key := m.sectionKey(section.role)
		if err := m.viewport.SetSection(key, section.markdown); err != nil {
			return err
		}
		current[key] = struct{}{}
	}
	for key := range m.live {
		if _, ok := current[key]; ok {
			continue
		}
		if err := m.viewport.DeleteSection(key); err != nil {
			return err
		}
	}
	m.live = current
	m.activeRole = ""
	m.status = "complete"
	return m.clearCursor()
}

func (m *channelModel) sectionKey(role string) string {
	return fmt.Sprintf("turn-%d-%s", m.turn, role)
}

func pumpChannelFrames(ctx context.Context, program *tea.Program, stream goclient.JSONStream) error {
	first := true
	for {
		frame, err := recvChannelFrame(ctx, stream)
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, io.EOF) {
				return err
			}
			program.Send(channelErrMsg{err: err})
			return err
		}

		title := "recv"
		if first {
			title = "session"
			first = false
		}
		program.Send(channelFrameMsg{title: title, frame: frame})
	}
}

func channelBlinkCmd() tea.Cmd {
	return tea.Tick(channelBlinkInterval, func(time.Time) tea.Msg {
		return channelBlinkMsg{}
	})
}

func (m *channelModel) startStreaming() error {
	m.streaming = true
	m.spinnerFrame = 0
	m.activeRole = ""
	return nil
}

func (m *channelModel) clearCursor() error {
	m.streaming = false
	m.spinnerFrame = 0
	m.activeRole = ""
	return nil
}

func (m *channelModel) tickCursor() error {
	m.promptVisible = !m.promptVisible
	if !m.streaming {
		return nil
	}
	m.spinnerFrame = (m.spinnerFrame + 1) % len(channelSpinnerFrames)
	return nil
}

func (m *channelModel) spinnerGlyph() string {
	if len(channelSpinnerFrames) == 0 {
		return ""
	}
	index := m.spinnerFrame % len(channelSpinnerFrames)
	if index < 0 {
		index = 0
	}
	return channelSpinnerFrames[index]
}

func (m *channelModel) streamingLabel() string {
	switch m.activeRole {
	case schema.RoleThinking:
		return "thinking"
	case schema.RoleTool:
		return "calling tool"
	case schema.RoleAssistant:
		return "replying"
	default:
		return "busy"
	}
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

func channelPayload(text string) (json.RawMessage, error) {
	data, err := json.Marshal(schema.SessionChannelRequest{Text: text})
	if err != nil {
		return nil, err
	}
	return json.RawMessage(data), nil
}

func recvChannelFrame(ctx context.Context, stream goclient.JSONStream) (json.RawMessage, error) {
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case frame, ok := <-stream.Recv():
			if !ok {
				return nil, io.EOF
			}
			if frame == nil {
				continue
			}
			return frame, nil
		}
	}
}

func channelFrameMarkdown(title string, frame json.RawMessage) (string, error) {
	formatted, err := formatChannelFrame(frame)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("### %s\n\n```json\n%s\n```", strings.Title(title), formatted), nil
}

func channelRequestMarkdown(text string) string {
	return fmt.Sprintf("### User\n\n%s", text)
}

func channelDeltaMarkdown(role, text string) string {
	return fmt.Sprintf("### %s\n\n%s", strings.Title(role), text)
}

func channelErrorMarkdown(errFrame httpresponse.ErrResponse) string {
	markdown := fmt.Sprintf("### Error\n\n%d %s", errFrame.Code, errFrame.Reason)
	if errFrame.Detail == nil {
		return markdown
	}
	detail, err := json.Marshal(errFrame.Detail)
	if err != nil {
		return markdown
	}
	formatted, err := formatChannelFrame(detail)
	if err != nil {
		return markdown
	}
	return markdown + fmt.Sprintf("\n\n```json\n%s\n```", formatted)
}

func channelResponseMarkdown(response schema.ChatResponse) (string, error) {
	sections, err := channelResponseSections(response)
	if err != nil {
		return "", err
	}
	parts := make([]string, 0, len(sections))
	for _, section := range sections {
		parts = append(parts, section.markdown)
	}
	return strings.Join(parts, "\n\n"), nil
}

func channelResponseSections(response schema.ChatResponse) ([]channelResponseSection, error) {
	var thinking []string
	var content []string
	for _, block := range response.Content {
		switch {
		case block.Text != nil:
			content = append(content, *block.Text)
		case block.Thinking != nil:
			thinking = append(thinking, *block.Thinking)
		default:
			raw, err := json.Marshal(response)
			if err != nil {
				return nil, err
			}
			markdown, err := channelFrameMarkdown("response", raw)
			if err != nil {
				return nil, err
			}
			return []channelResponseSection{{role: response.Role, markdown: markdown}}, nil
		}
	}
	if len(thinking) == 0 && len(content) == 0 {
		raw, err := json.Marshal(response)
		if err != nil {
			return nil, err
		}
		markdown, err := channelFrameMarkdown("response", raw)
		if err != nil {
			return nil, err
		}
		return []channelResponseSection{{role: response.Role, markdown: markdown}}, nil
	}

	sections := make([]channelResponseSection, 0, 2)
	if len(thinking) > 0 {
		sections = append(sections, channelResponseSection{
			role:     "thinking",
			markdown: channelDeltaMarkdown("thinking", strings.Join(thinking, "\n\n")),
		})
	}
	if len(content) > 0 {
		role := response.Role
		markdown := fmt.Sprintf("### %s\n\n%s", strings.Title(role), strings.Join(content, "\n\n"))
		if result := response.Result.String(); result != "unknown" {
			markdown += fmt.Sprintf("\n\n_Result: %s_", result)
		}
		sections = append(sections, channelResponseSection{role: role, markdown: markdown})
	}
	return sections, nil
}

func decodeChannelErrorFrame(raw json.RawMessage) (httpresponse.ErrResponse, bool) {
	var errFrame httpresponse.ErrResponse
	if err := json.Unmarshal(raw, &errFrame); err != nil {
		return httpresponse.ErrResponse{}, false
	}
	if errFrame.Code == 0 {
		return httpresponse.ErrResponse{}, false
	}
	return errFrame, true
}

func decodeChannelDeltaFrame(raw json.RawMessage) (schema.StreamDelta, bool) {
	var delta schema.StreamDelta
	if err := json.Unmarshal(raw, &delta); err != nil {
		return schema.StreamDelta{}, false
	}
	if delta.Role == "" || delta.Text == "" {
		return schema.StreamDelta{}, false
	}
	return delta, true
}

func decodeChannelResponseFrame(raw json.RawMessage) (schema.ChatResponse, bool) {
	var response schema.ChatResponse
	if err := json.Unmarshal(raw, &response); err != nil {
		return schema.ChatResponse{}, false
	}
	if response.Role == "" || len(response.Content) == 0 {
		return schema.ChatResponse{}, false
	}
	return response, true
}

func formatChannelFrame(frame json.RawMessage) (string, error) {
	var buf bytes.Buffer
	if err := json.Indent(&buf, frame, "", "  "); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func trimLastRune(value string) string {
	runes := []rune(value)
	if len(runes) == 0 {
		return ""
	}
	return string(runes[:len(runes)-1])
}
