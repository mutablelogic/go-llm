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
	"time"

	"github.com/mutablelogic/go-llm/pkg/opt"
	"github.com/mutablelogic/go-llm/kernel/schema"
	"github.com/mutablelogic/go-llm/pkg/ui"
	uitable "github.com/mutablelogic/go-llm/pkg/ui/table"
)

// Client is the minimal API surface needed by the command handler.
// *httpclient.Client satisfies this interface.
type Client interface {
	GetSession(ctx context.Context, id string) (*schema.Session, error)
	CreateSession(ctx context.Context, meta schema.SessionMeta) (*schema.Session, error)
	UpdateSession(ctx context.Context, id string, meta schema.SessionMeta) (*schema.Session, error)
	DeleteSession(ctx context.Context, id string) error
	ListSessions(ctx context.Context, opts ...opt.Opt) (*schema.SessionList, error)
	ListModels(ctx context.Context, opts ...opt.Opt) (*schema.ModelList, error)
	ListTools(ctx context.Context, opts ...opt.Opt) (*schema.ToolList, error)
	ListAgents(ctx context.Context, opts ...opt.Opt) (*schema.ListAgentResponse, error)
	GetAgent(ctx context.Context, id string) (*schema.Agent, error)
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

func ptrIfNonEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func formatGeneratorModel(generator schema.GeneratorMeta) string {
	var model string
	if generator.Model != nil {
		model = *generator.Model
	}
	if generator.Provider != nil && *generator.Provider != "" {
		model = *generator.Provider + "/" + model
	}
	return model
}

func formatSessionTime(value *time.Time) string {
	if value == nil {
		return "-"
	}
	return value.Format("2006-01-02 15:04:05")
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
	case "agents":
		return h.cmdAgents(ctx, evt)
	case "agent":
		return h.cmdAgent(ctx, evt)
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
		model := formatGeneratorModel(session.GeneratorMeta)
		return evt.Context.SendText(ctx, fmt.Sprintf("Current model: %s", model))
	}
	session, err := h.client.GetSession(ctx, *sessionID)
	if err != nil {
		return err
	}
	generator := session.GeneratorMeta
	arg := strings.Join(evt.Args, " ")
	var provider, newModel string
	if parts := strings.SplitN(arg, "/", 2); len(parts) == 2 {
		provider = parts[0]
		newModel = parts[1]
	} else {
		newModel = arg
	}
	generator.Provider = ptrIfNonEmpty(provider)
	generator.Model = &newModel
	if _, err := h.client.UpdateSession(ctx, *sessionID, schema.SessionMeta{
		GeneratorMeta: generator,
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
		generator := session.GeneratorMeta
		buf.WriteString("| | |\n|---|---|") // header row (empty) + separator
		buf.WriteString(fmt.Sprintf("\n| **Session** | %s |", session.ID))
		if session.Title != nil && *session.Title != "" {
			buf.WriteString(fmt.Sprintf("\n| **Title** | %s |", *session.Title))
		}
		model := formatGeneratorModel(generator)
		if model != "" {
			buf.WriteString(fmt.Sprintf("\n| **Model** | %s |", model))
		}
		if generator.SystemPrompt != nil && *generator.SystemPrompt != "" {
			buf.WriteString(fmt.Sprintf("\n| **System** | %s |", *generator.SystemPrompt))
		}
		if generator.Thinking != nil {
			buf.WriteString(fmt.Sprintf("\n| **Thinking** | %v |", *generator.Thinking))
		}
		if len(session.Tags) > 0 {
			tags := append([]string(nil), session.Tags...)
			sort.Strings(tags)
			buf.WriteString(fmt.Sprintf("\n| **Tags** | %s |", strings.Join(tags, ", ")))
		}
		buf.WriteString(fmt.Sprintf("\n| **Created** | %s |", session.CreatedAt.Format("2006-01-02 15:04:05")))
		buf.WriteString(fmt.Sprintf("\n| **Modified** | %s |", formatSessionTime(session.ModifiedAt)))
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

	model := formatGeneratorModel(session.GeneratorMeta)
	return evt.Context.SendText(ctx, fmt.Sprintf("Switched to session: %s (%s)", newSessionID, model))
}

func (h *Handler) cmdSessions(ctx context.Context, evt ui.Event, sessionID *string) error {
	resp, err := h.client.ListSessions(ctx)
	if err != nil {
		return err
	}
	if len(resp.Body) == 0 {
		return evt.Context.SendText(ctx, "No sessions found")
	}
	return evt.Context.SendText(ctx, uitable.RenderMarkdown(schema.SessionTable{
		Sessions:       resp.Body,
		CurrentSession: *sessionID,
	}))
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
		generator := current.GeneratorMeta
		if generator.Provider != nil {
			provider = *generator.Provider
		}
		if generator.Model != nil {
			model = *generator.Model
		}
	}
	var g schema.GeneratorMeta
	if provider != "" {
		g.Provider = &provider
	}
	if model != "" {
		g.Model = &model
	}
	generator := g
	meta := schema.SessionMeta{
		GeneratorMeta: generator,
	}
	// Merge frontend-specific metadata (e.g. Telegram tags/title).
	if h.hooks != nil {
		if extra := h.hooks.ResetMeta(); extra != nil {
			meta.Title = extra.Title
			meta.Tags = append([]string(nil), extra.Tags...)
			generator = schema.MergeGeneratorMeta(generator, extra.GeneratorMeta)
			meta.GeneratorMeta = generator
		}
	}
	session, err := h.client.CreateSession(ctx, meta)
	if err != nil {
		return err
	}
	*sessionID = session.ID.String()
	if h.hooks != nil {
		h.hooks.OnSessionChanged(session.ID.String())
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
		if session.Title == nil || *session.Title == "" {
			return evt.Context.SendText(ctx, "Session has no title")
		}
		return evt.Context.SendText(ctx, fmt.Sprintf("Session title: %s", *session.Title))
	}
	name := strings.Join(evt.Args, " ")
	if _, err := h.client.UpdateSession(ctx, *sessionID, schema.SessionMeta{
		Title: &name,
	}); err != nil {
		return err
	}
	return evt.Context.SendText(ctx, fmt.Sprintf("Session retitled to: %s", name))
}

func (h *Handler) cmdSystem(ctx context.Context, evt ui.Event, sessionID *string) error {
	session, err := h.client.GetSession(ctx, *sessionID)
	if err != nil {
		return err
	}
	generator := session.GeneratorMeta
	if len(evt.Args) == 0 {
		if generator.SystemPrompt == nil || *generator.SystemPrompt == "" {
			return evt.Context.SendText(ctx, "No system prompt set")
		}
		return evt.Context.SendText(ctx, fmt.Sprintf("System prompt: %s", *generator.SystemPrompt))
	}
	prompt := strings.Join(evt.Args, " ")
	generator.SystemPrompt = &prompt
	if _, err := h.client.UpdateSession(ctx, *sessionID, schema.SessionMeta{
		GeneratorMeta: generator,
	}); err != nil {
		return err
	}
	return evt.Context.SendText(ctx, "System prompt updated")
}

func (h *Handler) cmdThinking(ctx context.Context, evt ui.Event, sessionID *string) error {
	session, err := h.client.GetSession(ctx, *sessionID)
	if err != nil {
		return err
	}
	generator := session.GeneratorMeta
	if len(evt.Args) == 0 {
		if generator.Thinking == nil {
			return evt.Context.SendText(ctx, "Thinking: not set")
		}
		return evt.Context.SendText(ctx, fmt.Sprintf("Thinking: %v", *generator.Thinking))
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
	generator.Thinking = &enabled
	if _, err := h.client.UpdateSession(ctx, *sessionID, schema.SessionMeta{
		GeneratorMeta: generator,
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
	return evt.Context.SendText(ctx, uitable.RenderMarkdown(schema.ToolTable(resp.Body)))
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
	return evt.Context.SendText(ctx, uitable.RenderMarkdown(schema.ModelTable{
		Models: resp.Body,
	}))
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
		if len(session.Tags) == 0 {
			return evt.Context.SendText(ctx, "No tags set")
		}
		tags := append([]string(nil), session.Tags...)
		sort.Strings(tags)
		return evt.Context.SendText(ctx, strings.Join(tags, "\n"))
	}
	tags := append([]string(nil), evt.Args...)
	if _, err := h.client.UpdateSession(ctx, *sessionID, schema.SessionMeta{
		Tags: tags,
	}); err != nil {
		return err
	}
	sort.Strings(tags)
	return evt.Context.SendText(ctx, fmt.Sprintf("Tags updated: %s", strings.Join(tags, ", ")))
}

func (h *Handler) cmdAgents(ctx context.Context, evt ui.Event) error {
	resp, err := h.client.ListAgents(ctx)
	if err != nil {
		return err
	}
	if len(resp.Body) == 0 {
		return evt.Context.SendText(ctx, "No agents found")
	}
	return evt.Context.SendText(ctx, uitable.RenderMarkdown(schema.AgentTable(resp.Body)))
}

func (h *Handler) cmdAgent(ctx context.Context, evt ui.Event) error {
	if len(evt.Args) == 0 {
		return fmt.Errorf("usage: /agent <name>")
	}
	id := evt.Args[0]
	a, err := h.client.GetAgent(ctx, id)
	if err != nil {
		return fmt.Errorf("agent %q not found", id)
	}
	var buf strings.Builder
	buf.WriteString("| | |\n|---|---|")
	buf.WriteString(fmt.Sprintf("\n| **Name** | %s |", a.Name))
	if a.Title != "" {
		buf.WriteString(fmt.Sprintf("\n| **Title** | %s |", a.Title))
	}
	if a.Description != "" {
		buf.WriteString(fmt.Sprintf("\n| **Description** | %s |", a.Description))
	}
	buf.WriteString(fmt.Sprintf("\n| **Version** | %d |", a.Version))
	if a.Model != nil && *a.Model != "" {
		model := *a.Model
		if a.Provider != nil && *a.Provider != "" {
			model = *a.Provider + "/" + model
		}
		buf.WriteString(fmt.Sprintf("\n| **Model** | %s |", model))
	}
	if len(a.Tools) > 0 {
		buf.WriteString(fmt.Sprintf("\n| **Tools** | %s |", strings.Join(a.Tools, ", ")))
	}
	if len(a.Format) > 0 {
		buf.WriteString("\n| **Format** | (JSON schema) |")
	}
	if len(a.Input) > 0 {
		buf.WriteString("\n| **Input** | (JSON schema) |")
	}
	buf.WriteString(fmt.Sprintf("\n| **Created** | %s |", a.Created.Format("2006-01-02 15:04:05")))
	return evt.Context.SendText(ctx, buf.String())
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
		"/name [title]           - Show or set the session title\n" +
		"/system [prompt]        - Show or set the system prompt\n" +
		"/thinking [on|off]      - Show or toggle thinking mode\n" +
		"/tools                  - List available tools\n" +
		"/agents                 - List available agents\n" +
		"/agent <name>           - Show agent details\n" +
		"/label [tag ...]        - Show or set session tags\n" +
		"/help                   - Show this help\n" +
		"```"
	return evt.Context.SendText(ctx, help)
}
