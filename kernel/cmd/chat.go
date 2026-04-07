package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	// Packages
	uuid "github.com/google/uuid"
	otel "github.com/mutablelogic/go-client/pkg/otel"
	httpclient "github.com/mutablelogic/go-llm/kernel/httpclient"
	opt "github.com/mutablelogic/go-llm/pkg/opt"
	schema "github.com/mutablelogic/go-llm/kernel/schema"
	tui "github.com/mutablelogic/go-llm/pkg/tui"
	server "github.com/mutablelogic/go-server"
	attribute "go.opentelemetry.io/otel/attribute"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type ChatCommands struct {
	Chat ChatCommand `cmd:"" name:"chat" help:"Send a message within an existing session." group:"RESPONSES"`
}

type ChatCommand struct {
	Session       uuid.UUID `name:"session" help:"Session ID (defaults to the stored current session)" optional:""`
	Text          string    `arg:"" help:"User input text"`
	Tools         []string  `name:"tool" help:"Tool names to include (may be repeated; nil means all, empty means none)" optional:""`
	MaxIterations uint      `name:"max-iterations" help:"Maximum tool-calling iterations (0 uses default)" optional:""`
	SystemPrompt  string    `name:"system-prompt" help:"Per-request system prompt appended to the session prompt" optional:""`
	Stream        bool      `name:"stream" help:"Stream the response as it is generated." default:"true" negatable:""`
	Out           string    `name:"out" type:"dir" help:"Path to write response attachments (defaults to stdout)" optional:""`
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

func (cmd *ChatCommand) Run(ctx server.Cmd) (err error) {
	if cmd.Session == uuid.Nil {
		if value := ctx.GetString("session"); value != "" {
			session, err := uuid.Parse(value)
			if err != nil {
				return fmt.Errorf("invalid stored session %q: %w", value, err)
			}
			cmd.Session = session
		}
	}
	if cmd.Session == uuid.Nil {
		return fmt.Errorf("session is required (set with --session or store a default)")
	}
	if err := ctx.Set("session", cmd.Session.String()); err != nil {
		return err
	}

	req := cmd.request()

	return WithClient(ctx, func(client *httpclient.Client, _ string) error {
		parent, endSpan := otel.StartSpan(ctx.Tracer(), ctx.Context(), "ChatCommand",
			attribute.String("request", req.String()),
		)
		defer func() { endSpan(err) }()

		widget := tui.Markdown(markdownOptsForStdout()...)
		streamRenderer := newMarkdownStream(os.Stdout, widget)
		var streamFn opt.StreamFn
		if cmd.Stream && !ctx.IsDebug() {
			streamFn = func(role, text string) {
				if role == schema.RoleAssistant {
					_ = streamRenderer.Append(text)
				}
			}
		}

		response, err := client.Chat(parent, req, streamFn)
		if err != nil {
			return err
		}

		if ctx.IsDebug() {
			fmt.Println(response)
			return nil
		}

		text := chatResponseText(response)
		attachments := chatResponseAttachments(response)
		if len(attachments) > 0 {
			out, err := cmd.outputFolder(ctx.Name())
			if err != nil {
				return err
			}
			for index, attachment := range attachments {
				target, err := writeAskResponseAttachment(attachment, out, index)
				if err != nil {
					return err
				}
				text += fmt.Sprintf("\n- [Attachment %d](%s)\n", index+1, target)
			}
		}

		if cmd.Stream {
			return streamRenderer.Finish(text)
		}
		return writeMarkdown(os.Stdout, widget, text)
	})
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

func (cmd ChatCommand) request() schema.ChatRequest {
	return schema.ChatRequest{
		Session:       cmd.Session,
		Text:          cmd.Text,
		Tools:         cmd.Tools,
		MaxIterations: cmd.MaxIterations,
		SystemPrompt:  cmd.SystemPrompt,
	}
}

func (cmd ChatCommand) outputFolder(defaultDir string) (string, error) {
	out := cmd.Out
	if out == "" {
		cache, err := os.UserCacheDir()
		if err != nil {
			return "", fmt.Errorf("getting user cache directory: %w", err)
		}
		out = filepath.Join(cache, defaultDir)
	}

	if err := os.MkdirAll(out, 0o755); err != nil {
		return "", fmt.Errorf("creating output directory: %w", err)
	}

	return out, nil
}

func chatResponseText(response *schema.ChatResponse) string {
	if response == nil {
		return ""
	}

	var builder strings.Builder
	for _, block := range response.Content {
		if block.Text != nil {
			builder.WriteString(*block.Text)
		}
	}

	return builder.String()
}

func chatResponseAttachments(response *schema.ChatResponse) []*schema.Attachment {
	if response == nil {
		return nil
	}

	attachments := make([]*schema.Attachment, 0)
	for _, block := range response.Content {
		if block.Attachment != nil {
			attachments = append(attachments, block.Attachment)
		}
	}

	return attachments
}
