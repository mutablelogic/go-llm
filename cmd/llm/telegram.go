package main

import (
	"context"
	"fmt"
	"io"
	"sync"

	// Packages
	otel "github.com/mutablelogic/go-client/pkg/otel"
	httpclient "github.com/mutablelogic/go-llm/pkg/httpclient"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	ui "github.com/mutablelogic/go-llm/pkg/ui"
	uicmd "github.com/mutablelogic/go-llm/pkg/ui/command"
	telegram "github.com/mutablelogic/go-llm/pkg/ui/telegram"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type TelegramCommands struct {
	Telegram TelegramCommand `cmd:"" name:"telegram" help:"Run as a Telegram bot." group:"SERVER"`
}

type TelegramCommand struct {
	Token          string   `name:"token" env:"TELEGRAM_TOKEN" help:"Telegram Bot API token" required:""`
	Model          string   `name:"model" help:"Default model for new sessions" optional:""`
	SystemPrompt   string   `name:"system-prompt" help:"System prompt for new sessions" optional:""`
	Thinking       *bool    `name:"thinking" negatable:"" help:"Enable thinking/reasoning" optional:""`
	ThinkingBudget uint     `name:"thinking-budget" help:"Thinking token budget" optional:""`
	Tool           []string `name:"tool" help:"Tool names to include (may be repeated; empty means all)" optional:""`

	// sessions caches conversation-ID → session-ID mappings so we don't
	// query the API on every message.
	mu       sync.Mutex        `kong:"-"`
	sessions map[string]string `kong:"-"`
}

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

const telegramChatLabel = "telegram-chat"

// telegramSystemSuffix is appended to the user's system prompt to guide
// the model toward Telegram-friendly formatting.
const telegramSystemSuffix = `Format responses for Telegram messenger. ` +
	`Supported formatting: bold, italic, underline, strikethrough, ` +
	`inline code, code blocks, hyperlinks, and blockquotes. ` +
	`NEVER use markdown headings (#), markdown tables, or markdown list syntax (- or *). ` +
	`Use bullet characters (•) for lists instead. ` +
	`Keep responses concise and well-structured for mobile reading.`

// telegramHooks implements command.Hooks for the Telegram frontend.
type telegramHooks struct {
	cmd            *TelegramCommand
	globals        *Globals
	client         *httpclient.Client
	conversationID string
	uctx           ui.Context
}

func (h *telegramHooks) OnSessionChanged(sessionID string) {
	h.cmd.mu.Lock()
	h.cmd.sessions[h.conversationID] = sessionID
	h.cmd.mu.Unlock()
}

func (h *telegramHooks) OnSessionReset() {
	// Nothing extra needed — OnSessionChanged already updated the cache.
}

///////////////////////////////////////////////////////////////////////////////
// COMMANDS

func (cmd *TelegramCommand) Run(ctx *Globals) (err error) {
	client, err := ctx.Client()
	if err != nil {
		return err
	}

	// OTEL
	parent, endSpan := otel.StartSpan(ctx.tracer, ctx.ctx, "TelegramCommand")
	defer func() { endSpan(err) }()

	// Resolve default model
	model := cmd.Model
	if model == "" {
		model = ctx.defaults.GetString("model")
	}
	if model == "" {
		return fmt.Errorf("model is required (set with --model or store a default)")
	}
	cmd.Model = model

	// Create Telegram bot UI
	bot, err := telegram.New(cmd.Token)
	if err != nil {
		return err
	}
	defer bot.Close()

	cmd.sessions = make(map[string]string)

	ctx.logger.Print(parent, "Telegram bot started")

	// Event loop
	for {
		evt, err := bot.Receive(parent)
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}

		switch evt.Type {
		case ui.EventText:
			sessionID, err := cmd.resolveSession(parent, ctx, client, evt.Context)
			if err != nil {
				evt.Context.SendText(parent, fmt.Sprintf("Error: %v", err))
				continue
			}
			if err := cmd.handleChat(parent, evt, client, sessionID); err != nil {
				evt.Context.SendText(parent, fmt.Sprintf("Error: %v", err))
			}
		case ui.EventCommand:
			sessionID, err := cmd.resolveSession(parent, ctx, client, evt.Context)
			if err != nil {
				evt.Context.SendText(parent, fmt.Sprintf("Error: %v", err))
				continue
			}
			cmdHandler := uicmd.New(client, &telegramHooks{
				cmd:            cmd,
				globals:        ctx,
				client:         client,
				conversationID: evt.Context.ConversationID(),
				uctx:           evt.Context,
			})
			if err := cmdHandler.Handle(parent, evt, &sessionID); err != nil {
				evt.Context.SendText(parent, fmt.Sprintf("Error: %v", err))
			}
		}
	}
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// resolveSession finds or creates a session for the given conversation.
// Sessions are looked up by the label "telegram-chat:<conversation_id>".
func (cmd *TelegramCommand) resolveSession(ctx context.Context, globals *Globals, client *httpclient.Client, uctx ui.Context) (string, error) {
	conversationID := uctx.ConversationID()

	// Fast path: check in-memory cache.
	cmd.mu.Lock()
	if id, ok := cmd.sessions[conversationID]; ok {
		cmd.mu.Unlock()
		return id, nil
	}
	cmd.mu.Unlock()

	// Look up an existing session by label.
	resp, err := client.ListSessions(ctx, httpclient.WithLabel(telegramChatLabel, conversationID))
	if err != nil {
		return "", fmt.Errorf("looking up session: %w", err)
	}
	if len(resp.Body) > 0 {
		id := resp.Body[0].ID
		cmd.mu.Lock()
		cmd.sessions[conversationID] = id
		cmd.mu.Unlock()
		return id, nil
	}

	// No session yet — create one.
	provider := globals.defaults.GetString("provider")
	session, err := client.CreateSession(ctx, schema.SessionMeta{
		GeneratorMeta: schema.GeneratorMeta{
			Provider:       provider,
			Model:          cmd.Model,
			SystemPrompt:   cmd.SystemPrompt,
			Thinking:       cmd.Thinking,
			ThinkingBudget: cmd.ThinkingBudget,
		},
		Name: uctx.UserName(),
		Labels: map[string]string{
			telegramChatLabel: conversationID,
		},
	})
	if err != nil {
		return "", fmt.Errorf("creating session: %w", err)
	}

	cmd.mu.Lock()
	cmd.sessions[conversationID] = session.ID
	cmd.mu.Unlock()

	return session.ID, nil
}

func (cmd *TelegramCommand) handleChat(ctx context.Context, evt ui.Event, client *httpclient.Client, sessionID string) error {
	evt.Context.SetTyping(ctx, true)
	evt.Context.StreamStart(ctx)

	opts := []httpclient.ChatOpt{
		httpclient.WithChatStream(func(role, text string) {
			evt.Context.StreamChunk(ctx, role, text)
		}),
	}

	req := schema.ChatRequest{
		Session:      sessionID,
		Text:         evt.Text,
		Tools:        cmd.Tool,
		SystemPrompt: telegramSystemSuffix,
	}

	_, err := client.Chat(ctx, req, opts...)

	evt.Context.StreamEnd(ctx)

	return err
}
