package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	// Packages
	otel "github.com/mutablelogic/go-client/pkg/otel"
	httpclient "github.com/mutablelogic/go-llm/pkg/httpclient"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	"github.com/mutablelogic/go-llm/pkg/ui"
	btui "github.com/mutablelogic/go-llm/pkg/ui/bubbletea"
	uicmd "github.com/mutablelogic/go-llm/pkg/ui/command"
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

// chatHooks implements command.Hooks for the bubbletea interactive session.
type chatHooks struct {
	globals *Globals
	tui     *btui.Terminal
	client  *httpclient.Client
}

func (h *chatHooks) OnSessionChanged(sessionID string) {
	_ = h.globals.defaults.Set("session", sessionID)

	// Restore chat history from the new session.
	if session, err := h.client.GetSession(context.Background(), sessionID); err == nil {
		h.tui.ClearHistory()
		for _, msg := range session.Messages {
			text := msg.Text()
			if text == "" {
				continue
			}
			h.tui.AppendHistory(msg.Role, text)
		}
	}
}

func (h *chatHooks) OnSessionReset() {
	h.tui.ClearHistory()
}

func (h *chatHooks) ResetMeta() *schema.SessionMeta {
	return nil
}

// runInteractive launches the bubbletea TUI for an interactive chat session.
func (cmd *ChatCommand) runInteractive(ctx context.Context, globals *Globals, client *httpclient.Client, sessionID string, baseOpts []httpclient.ChatOpt) error {
	tui, err := btui.New()
	if err != nil {
		return fmt.Errorf("initializing terminal UI: %w", err)
	}
	defer tui.Close()

	// Shared command handler with bubbletea-specific hooks.
	cmdHandler := uicmd.New(client, &chatHooks{
		globals: globals,
		tui:     tui,
		client:  client,
	})

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
			if err := cmd.handleCommand(ctx, evt, cmdHandler, &sessionID); err != nil {
				evt.Context.SendText(ctx, fmt.Sprintf("Error: %v", err))
			}
		case ui.EventText:
			if err := cmd.handleChat(ctx, evt, client, sessionID, baseOpts); err != nil {
				evt.Context.SendText(ctx, fmt.Sprintf("Error: %v", err))
			}
		}
	}
}

// handleCommand processes slash commands in interactive mode. It handles
// terminal-specific commands (/file, /url) locally and delegates the rest
// to the shared command handler.
func (cmd *ChatCommand) handleCommand(ctx context.Context, evt ui.Event, cmdHandler *uicmd.Handler, sessionID *string) error {
	switch evt.Command {
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

	case "help":
		// Show shared help first, then append terminal-only commands.
		if err := cmdHandler.Handle(ctx, evt, sessionID); err != nil {
			return err
		}
		extra := "Terminal-only commands:\n\n" +
			"```\n" +
			"/file <path>            - Attach file(s) to next message\n" +
			"/url <url>              - Attach a URL to next message\n" +
			"```"
		return evt.Context.SendText(ctx, extra)

	default:
		// Delegate to shared command handler.
		return cmdHandler.Handle(ctx, evt, sessionID)
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
