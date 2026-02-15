// Package command implements shared slash-command handling for chat UIs.
//
// The [Handler] processes commands like /model, /session, /reset, /help
// etc. and works with any [ui.Context] so the same logic can be used by
// terminal, Telegram, and other frontends.
package command

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/mutablelogic/go-llm/pkg/opt"
	"github.com/mutablelogic/go-llm/pkg/schema"
	"github.com/mutablelogic/go-llm/pkg/ui"
)

// Client is the minimal API surface needed by the command handler.
// *httpclient.Client satisfies this interface.
type Client interface {
	GetSession(ctx context.Context, id string) (*schema.Session, error)
	CreateSession(ctx context.Context, meta schema.SessionMeta) (*schema.Session, error)
	UpdateSession(ctx context.Context, id string, meta schema.SessionMeta) (*schema.Session, error)
	DeleteSession(ctx context.Context, id string) error
	ListSessions(ctx context.Context, opts ...opt.Opt) (*schema.ListSessionResponse, error)
	ListModels(ctx context.Context, opts ...opt.Opt) (*schema.ListModelsResponse, error)
	ListTools(ctx context.Context, opts ...opt.Opt) (*schema.ListToolResponse, error)
}

// Hooks allows frontends to inject UI-specific behaviour into certain
// commands. All methods are optional - nil Hooks is safe.
type Hooks interface {
	// OnSessionChanged is called when the active session changes
	// (via /session <id> or /reset). The frontend can persist the
	// new session ID or update its display.
	OnSessionChanged(sessionID string)

	// OnSessionReset is called after /reset creates a new session.
	// The frontend can clear its message history.
	OnSessionReset()

	// ResetMeta returns extra session metadata to include when /reset
	// creates a new session (e.g. labels, name). Return nil for defaults.
	ResetMeta() *schema.SessionMeta
}

// Handler processes slash commands against the API.
type Handler struct {
	client Client
	hooks  Hooks
}

// New creates a command handler with the given API client and optional hooks.
func New(client Client, hooks Hooks) *Handler {
	return &Handler{
		client: client,
		hooks:  hooks,
	}
}

// Handle processes a slash command event and returns an error if the command
// fails. The sessionID pointer is updated in place when the session changes.
func (h *Handler) Handle(ctx context.Context, evt ui.Event, sessionID *string) error {
	switch evt.Command {
	case "model":
		return h.cmdModel(ctx, evt, sessionID)
	case "session":
		return h.cmdSession(ctx, evt, sessionID)
	case "sessions":
		return h.cmdSessions(ctx, evt, sessionID)
	case "reset":
		return h.cmdReset(ctx, evt, sessionID)
	case "name":
		return h.cmdName(ctx, evt, sessionID)
	case "system":
		return h.cmdSystem(ctx, evt, sessionID)
	case "thinking":
		return h.cmdThinking(ctx, evt, sessionID)
	case "tools":
		return h.cmdTools(ctx, evt)
	case "models":
		return h.cmdModels(ctx, evt)
	case "providers":
		return h.cmdProviders(ctx, evt)
	case "label":
		return h.cmdLabel(ctx, evt, sessionID)
	case "delete":
		return h.cmdDelete(ctx, evt, sessionID)
	case "help":
		return h.cmdHelp(ctx, evt)
	default:
		return fmt.Errorf("unknown command: /%s (try /help)", evt.Command)
	}
}

func (h *Handler) cmdModel(ctx context.Context, evt ui.Event, sessionID *string) error {
	if len(evt.Args) == 0 {
		session, err := h.client.GetSession(ctx, *sessionID)
		if err != nil {
			return err
		}
		model := session.Model
		if session.Provider != "" {
			model = session.Provider + "/" + model
		}
		return evt.Context.SendText(ctx, fmt.Sprintf("Current model: %s", model))
	}
	arg := strings.Join(evt.Args, " ")
	var provider, newModel string
	if parts := strings.SplitN(arg, "/", 2); len(parts) == 2 {
		provider = parts[0]
		newModel = parts[1]
	} else {
		newModel = arg
	}
	if _, err := h.client.UpdateSession(ctx, *sessionID, schema.SessionMeta{
		GeneratorMeta: schema.GeneratorMeta{Provider: provider, Model: newModel},
	}); err != nil {
		return err
	}
	display := newModel
	if provider != "" {
		display = provider + "/" + newModel
	}
	return evt.Context.SendText(ctx, fmt.Sprintf("Switched to model: %s", display))
}

func (h *Handler) cmdSession(ctx context.Context, evt ui.Event, sessionID *string) error {
	if len(evt.Args) == 0 {
		session, err := h.client.GetSession(ctx, *sessionID)
		if err != nil {
			return err
		}
		var buf strings.Builder
		buf.WriteString("| | |\n|---|---|") // header row (empty) + separator
		buf.WriteString(fmt.Sprintf("\n| **Session** | %s |", session.ID))
		if session.Name != "" {
			buf.WriteString(fmt.Sprintf("\n| **Name** | %s |", session.Name))
		}
		model := session.Model
		if session.Provider != "" {
			model = session.Provider + "/" + model
		}
		if model != "" {
			buf.WriteString(fmt.Sprintf("\n| **Model** | %s |", model))
		}
		if session.SystemPrompt != "" {
			buf.WriteString(fmt.Sprintf("\n| **System** | %s |", session.SystemPrompt))
		}
		if session.Thinking != nil {
			buf.WriteString(fmt.Sprintf("\n| **Thinking** | %v |", *session.Thinking))
		}
		if len(session.Labels) > 0 {
			var labelParts []string
			for k, v := range session.Labels {
				labelParts = append(labelParts, fmt.Sprintf("%s=%s", k, v))
			}
			sort.Strings(labelParts)
			buf.WriteString(fmt.Sprintf("\n| **Labels** | %s |", strings.Join(labelParts, ", ")))
		}
		buf.WriteString(fmt.Sprintf("\n| **Messages** | %d |", len(session.Messages)))
		tokens := session.Tokens()
		if session.Overhead > 0 {
			buf.WriteString(fmt.Sprintf("\n| **Tokens** | %d (+%d overhead/turn) |", tokens, session.Overhead))
		} else {
			buf.WriteString(fmt.Sprintf("\n| **Tokens** | %d |", tokens))
		}
		buf.WriteString(fmt.Sprintf("\n| **Created** | %s |", session.Created.Format("2006-01-02 15:04:05")))
		buf.WriteString(fmt.Sprintf("\n| **Modified** | %s |", session.Modified.Format("2006-01-02 15:04:05")))
		return evt.Context.SendText(ctx, buf.String())
	}

	// Switch to a different session.
	newSessionID := evt.Args[0]
	session, err := h.client.GetSession(ctx, newSessionID)
	if err != nil {
		return fmt.Errorf("session %q not found", newSessionID)
	}
	*sessionID = newSessionID
	if h.hooks != nil {
		h.hooks.OnSessionChanged(newSessionID)
	}

	model := session.Model
	if session.Provider != "" {
		model = session.Provider + "/" + model
	}
	return evt.Context.SendText(ctx, fmt.Sprintf("Switched to session: %s (%s, %d msgs)", newSessionID, model, len(session.Messages)))
}

func (h *Handler) cmdSessions(ctx context.Context, evt ui.Event, sessionID *string) error {
	resp, err := h.client.ListSessions(ctx)
	if err != nil {
		return err
	}
	if len(resp.Body) == 0 {
		return evt.Context.SendText(ctx, "No sessions found")
	}
	var buf strings.Builder
	buf.WriteString("| | Session | Model | Msgs | Name |\n")
	buf.WriteString("|---|---------|-------|------|------|")
	for _, s := range resp.Body {
		model := s.Model
		if s.Provider != "" {
			model = s.Provider + "/" + model
		}
		marker := " "
		if s.ID == *sessionID {
			marker = "\u25b8"
		}
		buf.WriteString(fmt.Sprintf("\n| %s | %s | %s | %d | %s |", marker, s.ID, model, len(s.Messages), s.Name))
	}
	return evt.Context.SendText(ctx, buf.String())
}

func (h *Handler) cmdReset(ctx context.Context, evt ui.Event, sessionID *string) error {
	var provider, model string
	if len(evt.Args) > 0 {
		arg := strings.Join(evt.Args, " ")
		if parts := strings.SplitN(arg, "/", 2); len(parts) == 2 {
			provider = parts[0]
			model = parts[1]
		} else {
			model = arg
		}
	} else {
		current, err := h.client.GetSession(ctx, *sessionID)
		if err != nil {
			return err
		}
		provider = current.Provider
		model = current.Model
	}
	meta := schema.SessionMeta{
		GeneratorMeta: schema.GeneratorMeta{
			Provider: provider,
			Model:    model,
		},
	}
	// Merge frontend-specific metadata (e.g. Telegram labels/name).
	if h.hooks != nil {
		if extra := h.hooks.ResetMeta(); extra != nil {
			meta.Name = extra.Name
			meta.Labels = extra.Labels
			if extra.SystemPrompt != "" {
				meta.SystemPrompt = extra.SystemPrompt
			}
			if extra.Thinking != nil {
				meta.Thinking = extra.Thinking
			}
			if extra.ThinkingBudget > 0 {
				meta.ThinkingBudget = extra.ThinkingBudget
			}
		}
	}
	session, err := h.client.CreateSession(ctx, meta)
	if err != nil {
		return err
	}
	*sessionID = session.ID
	if h.hooks != nil {
		h.hooks.OnSessionChanged(session.ID)
		h.hooks.OnSessionReset()
	}
	display := model
	if provider != "" {
		display = provider + "/" + model
	}
	return evt.Context.SendText(ctx, fmt.Sprintf("New session %s (%s)", session.ID, display))
}

func (h *Handler) cmdName(ctx context.Context, evt ui.Event, sessionID *string) error {
	if len(evt.Args) == 0 {
		session, err := h.client.GetSession(ctx, *sessionID)
		if err != nil {
			return err
		}
		if session.Name == "" {
			return evt.Context.SendText(ctx, "Session has no name")
		}
		return evt.Context.SendText(ctx, fmt.Sprintf("Session name: %s", session.Name))
	}
	name := strings.Join(evt.Args, " ")
	if _, err := h.client.UpdateSession(ctx, *sessionID, schema.SessionMeta{
		Name: name,
	}); err != nil {
		return err
	}
	return evt.Context.SendText(ctx, fmt.Sprintf("Session renamed to: %s", name))
}

func (h *Handler) cmdSystem(ctx context.Context, evt ui.Event, sessionID *string) error {
	if len(evt.Args) == 0 {
		session, err := h.client.GetSession(ctx, *sessionID)
		if err != nil {
			return err
		}
		if session.SystemPrompt == "" {
			return evt.Context.SendText(ctx, "No system prompt set")
		}
		return evt.Context.SendText(ctx, fmt.Sprintf("System prompt: %s", session.SystemPrompt))
	}
	prompt := strings.Join(evt.Args, " ")
	if _, err := h.client.UpdateSession(ctx, *sessionID, schema.SessionMeta{
		GeneratorMeta: schema.GeneratorMeta{SystemPrompt: prompt},
	}); err != nil {
		return err
	}
	return evt.Context.SendText(ctx, "System prompt updated")
}

func (h *Handler) cmdThinking(ctx context.Context, evt ui.Event, sessionID *string) error {
	if len(evt.Args) == 0 {
		session, err := h.client.GetSession(ctx, *sessionID)
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
	if _, err := h.client.UpdateSession(ctx, *sessionID, schema.SessionMeta{
		GeneratorMeta: schema.GeneratorMeta{Thinking: &enabled},
	}); err != nil {
		return err
	}
	if enabled {
		return evt.Context.SendText(ctx, "Thinking enabled")
	}
	return evt.Context.SendText(ctx, "Thinking disabled")
}

func (h *Handler) cmdTools(ctx context.Context, evt ui.Event) error {
	resp, err := h.client.ListTools(ctx)
	if err != nil {
		return err
	}
	if len(resp.Body) == 0 {
		return evt.Context.SendText(ctx, "No tools available")
	}
	var buf strings.Builder
	buf.WriteString("| Tool | Description |\n")
	buf.WriteString("|------|-------------|")
	for _, t := range resp.Body {
		buf.WriteString(fmt.Sprintf("\n| %s | %s |", t.Name, t.Description))
	}
	return evt.Context.SendText(ctx, buf.String())
}

func (h *Handler) cmdModels(ctx context.Context, evt ui.Event) error {
	var opts []opt.Opt
	if len(evt.Args) > 0 {
		opts = append(opts, opt.SetString(opt.ProviderKey, evt.Args[0]))
	}
	resp, err := h.client.ListModels(ctx, opts...)
	if err != nil {
		return err
	}
	if len(resp.Body) == 0 {
		return evt.Context.SendText(ctx, "No models found")
	}
	var buf strings.Builder
	buf.WriteString("| Provider | Model | Description |\n")
	buf.WriteString("|----------|-------|-------------|")
	for _, m := range resp.Body {
		buf.WriteString(fmt.Sprintf("\n| %s | %s | %s |", m.OwnedBy, m.Name, m.Description))
	}
	return evt.Context.SendText(ctx, buf.String())
}

func (h *Handler) cmdProviders(ctx context.Context, evt ui.Event) error {
	resp, err := h.client.ListModels(ctx)
	if err != nil {
		return err
	}
	if len(resp.Provider) == 0 {
		return evt.Context.SendText(ctx, "No providers found")
	}
	return evt.Context.SendText(ctx, strings.Join(resp.Provider, "\n"))
}

func (h *Handler) cmdLabel(ctx context.Context, evt ui.Event, sessionID *string) error {
	if len(evt.Args) == 0 {
		session, err := h.client.GetSession(ctx, *sessionID)
		if err != nil {
			return err
		}
		if len(session.Labels) == 0 {
			return evt.Context.SendText(ctx, "No labels set")
		}
		var parts []string
		for k, v := range session.Labels {
			parts = append(parts, fmt.Sprintf("%s=%s", k, v))
		}
		sort.Strings(parts)
		return evt.Context.SendText(ctx, strings.Join(parts, "\n"))
	}
	labels := make(map[string]string)
	for _, arg := range evt.Args {
		k, v, ok := strings.Cut(arg, "=")
		if !ok || k == "" {
			return fmt.Errorf("invalid label %q (use key=value)", arg)
		}
		labels[k] = v
	}
	if _, err := h.client.UpdateSession(ctx, *sessionID, schema.SessionMeta{
		Labels: labels,
	}); err != nil {
		return err
	}
	var parts []string
	for k, v := range labels {
		parts = append(parts, fmt.Sprintf("%s=%s", k, v))
	}
	sort.Strings(parts)
	return evt.Context.SendText(ctx, fmt.Sprintf("Labels updated: %s", strings.Join(parts, ", ")))
}

func (h *Handler) cmdDelete(ctx context.Context, evt ui.Event, sessionID *string) error {
	if len(evt.Args) == 0 {
		return fmt.Errorf("usage: /delete <session-id>")
	}
	target := evt.Args[0]
	if target == *sessionID {
		return fmt.Errorf("cannot delete the current session (use /reset first)")
	}
	if err := h.client.DeleteSession(ctx, target); err != nil {
		return err
	}
	return evt.Context.SendText(ctx, fmt.Sprintf("Deleted session %s", target))
}

func (h *Handler) cmdHelp(ctx context.Context, evt ui.Event) error {
	help := "Available commands:\n\n" +
		"```\n" +
		"/model [provider/model] - Show or switch the current model\n" +
		"/models [provider]      - List available models\n" +
		"/providers              - List available providers\n" +
		"/session [id]           - Show or switch the current session\n" +
		"/sessions               - List all sessions\n" +
		"/reset [provider/model] - Start a new session\n" +
		"/delete <session-id>    - Delete a session\n" +
		"/name [name]            - Show or set the session name\n" +
		"/system [prompt]        - Show or set the system prompt\n" +
		"/thinking [on|off]      - Show or toggle thinking mode\n" +
		"/tools                  - List available tools\n" +
		"/label [key=value ...]  - Show or set session labels\n" +
		"/help                   - Show this help\n" +
		"```"
	return evt.Context.SendText(ctx, help)
}
