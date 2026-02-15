package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	// Packages
	otel "github.com/mutablelogic/go-client/pkg/otel"
	httpclient "github.com/mutablelogic/go-llm/pkg/httpclient"
	opt "github.com/mutablelogic/go-llm/pkg/opt"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	"github.com/mutablelogic/go-llm/pkg/ui"
	btui "github.com/mutablelogic/go-llm/pkg/ui/bubbletea"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type ChatCommand struct {
	Text           string   `arg:"" help:"User input text (omit for interactive mode)" optional:""`
	Model          string   `name:"model" help:"Model name (used when creating a new session)" optional:""`
	Session        string   `name:"session" help:"Session ID (overrides stored default)" optional:""`
	SystemPrompt   string   `name:"system-prompt" help:"System prompt (used when creating a new session)" optional:""`
	Thinking       *bool    `name:"thinking" negatable:"" help:"Enable thinking/reasoning" optional:""`
	ThinkingBudget uint     `name:"thinking-budget" help:"Thinking token budget (required for Anthropic, optional for Google)" optional:""`
	File           []string `help:"Path or glob pattern for files to attach (may be repeated)" optional:""`
	URL            string   `help:"URL to attach as a reference" optional:""`
	Tool           []string `name:"tool" help:"Tool names to include (may be repeated; empty means all)" optional:""`
	New            bool     `name:"new" help:"Force creation of a new session" optional:""`

	// pendingOpts accumulates file/URL attachments set via /file and /url
	// commands in interactive mode, consumed on the next chat message.
	pendingOpts []httpclient.ChatOpt `kong:"-"`
}

///////////////////////////////////////////////////////////////////////////////
// COMMANDS

func (cmd *ChatCommand) Run(ctx *Globals) (err error) {
	client, err := ctx.Client()
	if err != nil {
		return err
	}

	// OTEL
	parent, endSpan := otel.StartSpan(ctx.tracer, ctx.ctx, "ChatCommand")
	defer func() { endSpan(err) }()

	// Ensure a session exists
	sessionID, err := cmd.ensureSession(parent, ctx, client)
	if err != nil {
		return err
	}

	// Build file attachment options
	opts, err := cmd.chatOpts()
	if err != nil {
		return err
	}

	// Interactive mode when no text argument is provided
	if cmd.Text == "" {
		return cmd.runInteractive(parent, ctx, client, sessionID, opts)
	}

	// Single-shot mode: always stream to stdout
	return cmd.runSingleShot(parent, ctx, client, sessionID, opts)
}

// ensureSession resolves or creates a session and persists it as the default.
func (cmd *ChatCommand) ensureSession(ctx context.Context, globals *Globals, client *httpclient.Client) (string, error) {
	// Determine session ID: explicit flag > stored default
	sessionID := cmd.Session
	if sessionID == "" && !cmd.New {
		sessionID = globals.defaults.GetString("session")
	}

	// Verify the session still exists; clear it if not
	if sessionID != "" {
		if _, err := client.GetSession(ctx, sessionID); err != nil {
			sessionID = ""
		}
	}

	// If we still have no session, create one
	if sessionID == "" {
		model := cmd.Model
		if model == "" {
			model = globals.defaults.GetString("model")
		}
		if model == "" {
			return "", fmt.Errorf("model is required to create a session (set with --model or store a default)")
		}

		provider := globals.defaults.GetString("provider")

		session, err := client.CreateSession(ctx, schema.SessionMeta{
			GeneratorMeta: schema.GeneratorMeta{
				Provider:       provider,
				Model:          model,
				SystemPrompt:   cmd.SystemPrompt,
				Thinking:       cmd.Thinking,
				ThinkingBudget: cmd.ThinkingBudget,
			},
		})
		if err != nil {
			return "", fmt.Errorf("creating session: %w", err)
		}
		sessionID = session.ID
	} else {
		// If reusing an existing session but parameters were changed, update it
		meta := schema.SessionMeta{
			GeneratorMeta: schema.GeneratorMeta{
				Model:          cmd.Model,
				SystemPrompt:   cmd.SystemPrompt,
				Thinking:       cmd.Thinking,
				ThinkingBudget: cmd.ThinkingBudget,
			},
		}
		if meta.Model != "" || meta.SystemPrompt != "" || meta.Thinking != nil || meta.ThinkingBudget > 0 {
			if _, err := client.UpdateSession(ctx, sessionID, meta); err != nil {
				return "", fmt.Errorf("updating session: %w", err)
			}
		}
	}

	// Persist the session ID as the current default
	if err := globals.defaults.Set("session", sessionID); err != nil {
		return "", err
	}

	return sessionID, nil
}

// chatOpts builds file/URL attachment options from the command flags.
func (cmd *ChatCommand) chatOpts() ([]httpclient.ChatOpt, error) {
	var opts []httpclient.ChatOpt
	for _, pattern := range cmd.File {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return nil, fmt.Errorf("invalid glob pattern %q: %w", pattern, err)
		}
		if len(matches) == 0 {
			return nil, fmt.Errorf("no files match %q", pattern)
		}
		for _, path := range matches {
			f, err := os.Open(path)
			if err != nil {
				return nil, err
			}
			// Note: files are closed when the process exits for CLI usage
			opts = append(opts, httpclient.WithChatFile(f.Name(), f))
		}
	}
	if cmd.URL != "" {
		opts = append(opts, httpclient.WithChatURL(cmd.URL))
	}
	return opts, nil
}

// runSingleShot sends a single message and streams the response to stdout.
func (cmd *ChatCommand) runSingleShot(ctx context.Context, globals *Globals, client *httpclient.Client, sessionID string, opts []httpclient.ChatOpt) error {
	// Always stream in single-shot mode
	var lastRole string
	opts = append(opts, httpclient.WithChatStream(func(role, text string) {
		if role != lastRole {
			if lastRole != "" {
				fmt.Println()
			}
			fmt.Print(role + ": ")
			lastRole = role
		}
		fmt.Print(text)
	}))

	req := schema.ChatRequest{
		Session: sessionID,
		Text:    cmd.Text,
		Tools:   cmd.Tool,
	}

	if _, err := client.Chat(ctx, req, opts...); err != nil {
		return err
	}
	fmt.Println()
	return nil
}

// runInteractive launches the bubbletea TUI for an interactive chat session.
func (cmd *ChatCommand) runInteractive(ctx context.Context, globals *Globals, client *httpclient.Client, sessionID string, baseOpts []httpclient.ChatOpt) error {
	tui, err := btui.New()
	if err != nil {
		return fmt.Errorf("initializing terminal UI: %w", err)
	}
	defer tui.Close()

	// Restore previous messages from the session
	if session, err := client.GetSession(ctx, sessionID); err == nil {
		for _, msg := range session.Messages {
			text := msg.Text()
			if text == "" {
				continue
			}
			tui.AppendHistory(msg.Role, text)
		}
	}

	for {
		evt, err := tui.Receive(ctx)
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}

		switch evt.Type {
		case ui.EventCommand:
			if err := cmd.handleCommand(ctx, evt, globals, client, &sessionID); err != nil {
				evt.Context.SendText(ctx, fmt.Sprintf("Error: %v", err))
			}
		case ui.EventText:
			if err := cmd.handleChat(ctx, evt, client, sessionID, baseOpts); err != nil {
				evt.Context.SendText(ctx, fmt.Sprintf("Error: %v", err))
			}
		}
	}
}

// handleCommand processes slash commands in interactive mode.
func (cmd *ChatCommand) handleCommand(ctx context.Context, evt ui.Event, globals *Globals, client *httpclient.Client, sessionID *string) error {
	switch evt.Command {
	case "model":
		if len(evt.Args) == 0 {
			// Query current model
			session, err := client.GetSession(ctx, *sessionID)
			if err != nil {
				return err
			}
			return evt.Context.SendText(ctx, fmt.Sprintf("Current model: %s", session.Model))
		}
		// Switch model - supports "provider/model" format
		arg := strings.Join(evt.Args, " ")
		var provider, newModel string
		if parts := strings.SplitN(arg, "/", 2); len(parts) == 2 {
			provider = parts[0]
			newModel = parts[1]
		} else {
			newModel = arg
		}
		if _, err := client.UpdateSession(ctx, *sessionID, schema.SessionMeta{
			GeneratorMeta: schema.GeneratorMeta{Provider: provider, Model: newModel},
		}); err != nil {
			return err
		}
		display := newModel
		if provider != "" {
			display = provider + "/" + newModel
		}
		return evt.Context.SendText(ctx, fmt.Sprintf("Switched to model: %s", display))

	case "session":
		if len(evt.Args) == 0 {
			session, err := client.GetSession(ctx, *sessionID)
			if err != nil {
				return err
			}
			var lines []string
			lines = append(lines, fmt.Sprintf("Session:  %s", session.ID))
			if session.Name != "" {
				lines = append(lines, fmt.Sprintf("Name:     %s", session.Name))
			}
			model := session.Model
			if session.Provider != "" {
				model = session.Provider + "/" + model
			}
			if model != "" {
				lines = append(lines, fmt.Sprintf("Model:    %s", model))
			}
			if session.SystemPrompt != "" {
				lines = append(lines, fmt.Sprintf("System:   %s", session.SystemPrompt))
			}
			if session.Thinking != nil {
				lines = append(lines, fmt.Sprintf("Thinking: %v", *session.Thinking))
			}
			lines = append(lines, fmt.Sprintf("Messages: %d", len(session.Messages)))
			tokens := session.Tokens()
			if session.Overhead > 0 {
				lines = append(lines, fmt.Sprintf("Tokens:   %d (+%d overhead/turn)", tokens, session.Overhead))
			} else {
				lines = append(lines, fmt.Sprintf("Tokens:   %d", tokens))
			}
			lines = append(lines, fmt.Sprintf("Created:  %s", session.Created.Format("2006-01-02 15:04:05")))
			lines = append(lines, fmt.Sprintf("Modified: %s", session.Modified.Format("2006-01-02 15:04:05")))
			return evt.Context.SendText(ctx, strings.Join(lines, "\n"))
		}
		// Switch to a different session
		newSessionID := evt.Args[0]
		if _, err := client.GetSession(ctx, newSessionID); err != nil {
			return fmt.Errorf("session %q not found", newSessionID)
		}
		*sessionID = newSessionID
		if err := globals.defaults.Set("session", newSessionID); err != nil {
			return err
		}
		return evt.Context.SendText(ctx, fmt.Sprintf("Switched to session: %s", newSessionID))

	case "new":
		model := globals.defaults.GetString("model")
		if len(evt.Args) > 0 {
			model = strings.Join(evt.Args, " ")
		}
		if model == "" {
			return fmt.Errorf("model is required (use /new <model>)")
		}
		provider := globals.defaults.GetString("provider")
		session, err := client.CreateSession(ctx, schema.SessionMeta{
			GeneratorMeta: schema.GeneratorMeta{
				Provider: provider,
				Model:    model,
			},
		})
		if err != nil {
			return err
		}
		*sessionID = session.ID
		if err := globals.defaults.Set("session", session.ID); err != nil {
			return err
		}
		return evt.Context.SendText(ctx, fmt.Sprintf("New session %s (model: %s)", session.ID, model))

	case "file":
		if len(evt.Args) == 0 {
			if len(cmd.pendingOpts) == 0 {
				return evt.Context.SendText(ctx, "No files attached. Usage: /file <path>")
			}
			return evt.Context.SendText(ctx, fmt.Sprintf("%d attachment(s) pending for next message", len(cmd.pendingOpts)))
		}
		for _, pattern := range evt.Args {
			matches, err := filepath.Glob(pattern)
			if err != nil {
				return fmt.Errorf("invalid glob pattern %q: %w", pattern, err)
			}
			if len(matches) == 0 {
				return fmt.Errorf("no files match %q", pattern)
			}
			for _, path := range matches {
				f, err := os.Open(path)
				if err != nil {
					return err
				}
				cmd.pendingOpts = append(cmd.pendingOpts, httpclient.WithChatFile(f.Name(), f))
				evt.Context.SendText(ctx, fmt.Sprintf("Attached: %s", path))
			}
		}
		return nil

	case "url":
		if len(evt.Args) == 0 {
			return evt.Context.SendText(ctx, "Usage: /url <url>")
		}
		u := evt.Args[0]
		cmd.pendingOpts = append(cmd.pendingOpts, httpclient.WithChatURL(u))
		return evt.Context.SendText(ctx, fmt.Sprintf("Attached URL: %s", u))

	case "system":
		if len(evt.Args) == 0 {
			session, err := client.GetSession(ctx, *sessionID)
			if err != nil {
				return err
			}
			if session.SystemPrompt == "" {
				return evt.Context.SendText(ctx, "No system prompt set")
			}
			return evt.Context.SendText(ctx, fmt.Sprintf("System prompt: %s", session.SystemPrompt))
		}
		prompt := strings.Join(evt.Args, " ")
		if _, err := client.UpdateSession(ctx, *sessionID, schema.SessionMeta{
			GeneratorMeta: schema.GeneratorMeta{SystemPrompt: prompt},
		}); err != nil {
			return err
		}
		return evt.Context.SendText(ctx, fmt.Sprintf("System prompt updated"))

	case "thinking":
		if len(evt.Args) == 0 {
			session, err := client.GetSession(ctx, *sessionID)
			if err != nil {
				return err
			}
			if session.Thinking == nil {
				return evt.Context.SendText(ctx, "Thinking: not set")
			}
			return evt.Context.SendText(ctx, fmt.Sprintf("Thinking: %v", *session.Thinking))
		}
		arg := strings.ToLower(evt.Args[0])
		var enabled bool
		switch arg {
		case "on", "true", "yes", "1":
			enabled = true
		case "off", "false", "no", "0":
			enabled = false
		default:
			return fmt.Errorf("usage: /thinking [on|off]")
		}
		if _, err := client.UpdateSession(ctx, *sessionID, schema.SessionMeta{
			GeneratorMeta: schema.GeneratorMeta{Thinking: &enabled},
		}); err != nil {
			return err
		}
		if enabled {
			return evt.Context.SendText(ctx, "Thinking enabled")
		}
		return evt.Context.SendText(ctx, "Thinking disabled")

	case "tools":
		resp, err := client.ListTools(ctx)
		if err != nil {
			return err
		}
		if len(resp.Body) == 0 {
			return evt.Context.SendText(ctx, "No tools available")
		}
		var lines []string
		for _, t := range resp.Body {
			line := t.Name
			if t.Description != "" {
				line += " - " + t.Description
			}
			lines = append(lines, line)
		}
		return evt.Context.SendText(ctx, strings.Join(lines, "\n"))

	case "sessions":
		resp, err := client.ListSessions(ctx)
		if err != nil {
			return err
		}
		if len(resp.Body) == 0 {
			return evt.Context.SendText(ctx, "No sessions found")
		}
		var lines []string
		for _, s := range resp.Body {
			model := s.Model
			if s.Provider != "" {
				model = s.Provider + "/" + model
			}
			line := fmt.Sprintf("%s  %s  %d msgs", s.ID, model, len(s.Messages))
			if s.Name != "" {
				line += fmt.Sprintf("  (%s)", s.Name)
			}
			if s.ID == *sessionID {
				line = "\033[1m" + line + "\033[0m"
			}
			lines = append(lines, line)
		}
		return evt.Context.SendText(ctx, strings.Join(lines, "\n"))

	case "name":
		if len(evt.Args) == 0 {
			session, err := client.GetSession(ctx, *sessionID)
			if err != nil {
				return err
			}
			if session.Name == "" {
				return evt.Context.SendText(ctx, "Session has no name")
			}
			return evt.Context.SendText(ctx, fmt.Sprintf("Session name: %s", session.Name))
		}
		name := strings.Join(evt.Args, " ")
		if _, err := client.UpdateSession(ctx, *sessionID, schema.SessionMeta{
			Name: name,
		}); err != nil {
			return err
		}
		return evt.Context.SendText(ctx, fmt.Sprintf("Session renamed to: %s", name))

	case "reset":
		// Get current session's model to create the new one with the same settings
		current, err := client.GetSession(ctx, *sessionID)
		if err != nil {
			return err
		}
		session, err := client.CreateSession(ctx, schema.SessionMeta{
			GeneratorMeta: schema.GeneratorMeta{
				Provider:     current.Provider,
				Model:        current.Model,
				SystemPrompt: current.SystemPrompt,
				Thinking:     current.Thinking,
			},
		})
		if err != nil {
			return err
		}
		*sessionID = session.ID
		if err := globals.defaults.Set("session", session.ID); err != nil {
			return err
		}
		cmd.pendingOpts = nil
		if tui, ok := evt.Context.(interface{ ClearHistory() }); ok {
			tui.ClearHistory()
		}
		model := session.Model
		if session.Provider != "" {
			model = session.Provider + "/" + model
		}
		return evt.Context.SendText(ctx, fmt.Sprintf("New session %s (%s)", session.ID, model))

	case "models":
		var opts []opt.Opt
		if len(evt.Args) > 0 {
			opts = append(opts, httpclient.WithProvider(evt.Args[0]))
		}
		resp, err := client.ListModels(ctx, opts...)
		if err != nil {
			return err
		}
		if len(resp.Body) == 0 {
			return evt.Context.SendText(ctx, "No models found")
		}
		var lines []string
		for _, m := range resp.Body {
			line := m.Name
			if m.OwnedBy != "" {
				line = m.OwnedBy + "/" + m.Name
			}
			if m.Description != "" {
				line += " - " + m.Description
			}
			lines = append(lines, line)
		}
		return evt.Context.SendText(ctx, strings.Join(lines, "\n"))

	case "providers":
		resp, err := client.ListModels(ctx)
		if err != nil {
			return err
		}
		if len(resp.Provider) == 0 {
			return evt.Context.SendText(ctx, "No providers found")
		}
		return evt.Context.SendText(ctx, strings.Join(resp.Provider, "\n"))

	case "help":
		help := "Available commands:\n" +
			"  /model [provider/model] - Show or switch the current model\n" +
			"  /models [provider]      - List available models\n" +
			"  /providers              - List available providers\n" +
			"  /session [id]           - Show or switch the current session\n" +
			"  /sessions               - List all sessions\n" +
			"  /new [model]            - Create a new session\n" +
			"  /name [name]            - Show or set the session name\n" +
			"  /system [prompt]        - Show or set the system prompt\n" +
			"  /thinking [on|off]      - Show or toggle thinking mode\n" +
			"  /tools                  - List available tools\n" +
			"  /file <path>            - Attach file(s) to next message\n" +
			"  /url <url>              - Attach a URL to next message\n" +
			"  /reset                  - New session and clear display\n" +
			"  /help                   - Show this help"
		return evt.Context.SendText(ctx, help)

	default:
		return fmt.Errorf("unknown command: /%s (try /help)", evt.Command)
	}
}

// handleChat sends a user message and streams the response back to the UI.
func (cmd *ChatCommand) handleChat(ctx context.Context, evt ui.Event, client *httpclient.Client, sessionID string, baseOpts []httpclient.ChatOpt) error {
	// Start streaming in the UI
	evt.Context.StreamStart(ctx)

	// Build streaming opts: stream all roles, inserting visual separation
	// when the role changes (e.g. assistant → tool → assistant).
	opts := append([]httpclient.ChatOpt{}, baseOpts...)

	// Consume any pending file/URL attachments from /file and /url commands
	if len(cmd.pendingOpts) > 0 {
		opts = append(opts, cmd.pendingOpts...)
		cmd.pendingOpts = nil
	}

	opts = append(opts, httpclient.WithChatStream(func(role, text string) {
		evt.Context.StreamChunk(ctx, role, text)
	}))

	req := schema.ChatRequest{
		Session: sessionID,
		Text:    evt.Text,
		Tools:   cmd.Tool,
	}

	_, err := client.Chat(ctx, req, opts...)

	// Finalise the stream (re-renders with full markdown)
	evt.Context.StreamEnd(ctx)

	return err
}
