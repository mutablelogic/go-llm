package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"

	// Packages
	otel "github.com/mutablelogic/go-client/pkg/otel"
	httpclient "github.com/mutablelogic/go-llm/pkg/httpclient-new"
	opt "github.com/mutablelogic/go-llm/pkg/opt"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	tui "github.com/mutablelogic/go-llm/pkg/tui"
	server "github.com/mutablelogic/go-server"
	types "github.com/mutablelogic/go-server/pkg/types"
	attribute "go.opentelemetry.io/otel/attribute"
	term "golang.org/x/term"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type AskCommands struct {
	Ask AskCommand `cmd:"" name:"ask" help:"Send a stateless text request to a model." group:"RESPOND"`
}

type AskCommand struct {
	schema.GeneratorMeta `embed:""`
	Text                 string   `arg:"" help:"User input text"`
	File                 []string `name:"file" help:"Path or glob pattern for files to attach (may be repeated)" optional:""`
	Stream               bool     `name:"stream" help:"Stream the response as it is generated." default:"true" negatable:""`
	Out                  string   `name:"out" type:"dir" help:"Path to write response attachments (defaults to stdout)" optional:""`
}

type markdownStream struct {
	widget interface {
		Write(io.Writer, string) (int, error)
	}
	writer io.Writer
	text   strings.Builder
	first  bool
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

func (cmd *AskCommand) Run(ctx server.Cmd) (err error) {
	if cmd.Model == "" {
		cmd.Model = ctx.GetString("model")
	}
	if cmd.Provider == "" {
		cmd.Provider = ctx.GetString("provider")
	}
	if cmd.Model == "" {
		return fmt.Errorf("model is required (set with --model or store a default)")
	}
	if err := ctx.Set("model", cmd.Model); err != nil {
		return err
	}
	if cmd.Provider != "" {
		if err := ctx.Set("provider", cmd.Provider); err != nil {
			return err
		}
	}

	req, err := cmd.request()
	if err != nil {
		return err
	}

	return WithClient(ctx, func(client *httpclient.Client, _ string) error {
		parent, endSpan := otel.StartSpan(ctx.Tracer(), ctx.Context(), "AskCommand",
			attribute.String("request", types.Stringify(req)),
		)
		defer func() { endSpan(err) }()

		widget := tui.Markdown(markdownOptsForStdout()...)
		streamRenderer := newMarkdownStream(os.Stdout, widget)
		var streamFn opt.StreamFn
		if cmd.Stream && !ctx.IsDebug() {
			streamFn = func(role, text string) {
				if role == schema.RoleAssistant {
					streamRenderer.Append(text)
				}
			}
		}

		response, err := client.Ask(parent, req, streamFn)
		if err != nil {
			return err
		}

		if ctx.IsDebug() {
			fmt.Println(response)
			return nil
		}

		text := askResponseText(response)
		if len(req.Format) > 0 {
			var raw json.RawMessage
			if err := json.Unmarshal([]byte(text), &raw); err == nil {
				if indented, err := json.MarshalIndent(raw, "", "  "); err == nil {
					fmt.Println(string(indented))
					return nil
				}
			}
		}

		attachments := askResponseAttachments(response)
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

func (cmd AskCommand) outputFolder(defaultDir string) (string, error) {
	out := cmd.Out
	if out == "" {
		cache, err := os.UserCacheDir()
		if err != nil {
			return "", fmt.Errorf("getting user cache directory: %w", err)
		} else {
			out = filepath.Join(cache, defaultDir)
		}
	}

	// Create output directory if it doesn't exist
	if err := os.MkdirAll(out, 0755); err != nil {
		return "", fmt.Errorf("creating output directory: %w", err)
	}

	// Return the output path
	return out, nil
}

func (cmd AskCommand) request() (schema.AskRequest, error) {
	req := schema.AskRequest{
		AskRequestCore: schema.AskRequestCore{
			GeneratorMeta: cmd.GeneratorMeta,
			Text:          cmd.Text,
		},
	}

	attachments, err := askAttachments(cmd.File)
	if err != nil {
		return schema.AskRequest{}, err
	}
	if len(attachments) > 0 {
		req.Attachments = attachments
	}

	return req, nil
}

func askAttachments(patterns []string) ([]schema.Attachment, error) {
	attachments := make([]schema.Attachment, 0, len(patterns))
	for _, pattern := range patterns {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return nil, fmt.Errorf("invalid glob pattern %q: %w", pattern, err)
		}
		if len(matches) == 0 {
			return nil, fmt.Errorf("no files match %q", pattern)
		}
		for _, path := range matches {
			data, err := os.ReadFile(path)
			if err != nil {
				return nil, fmt.Errorf("reading file %q: %w", path, err)
			}
			attachments = append(attachments, schema.Attachment{
				ContentType: http.DetectContentType(data),
				Data:        data,
				URL:         &url.URL{Scheme: "file", Path: path},
			})
		}
	}

	return attachments, nil
}

func askResponseText(response *schema.AskResponse) string {
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

func askResponseAttachments(response *schema.AskResponse) []*schema.Attachment {
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

func writeAskResponseAttachment(attachment *schema.Attachment, out string, index int) (*url.URL, error) {
	if attachment == nil {
		return nil, fmt.Errorf("attachment is nil")
	}

	target, err := askResponseAttachmentURL(attachment, out, index)
	if err != nil {
		return nil, err
	}
	if len(attachment.Data) > 0 {
		if err := os.WriteFile(target.Path, attachment.Data, 0o600); err != nil {
			return nil, fmt.Errorf("writing attachment %q: %w", target.Path, err)
		}
	}
	attachment.URL = target

	return target, nil
}

func askResponseAttachmentURL(attachment *schema.Attachment, out string, index int) (*url.URL, error) {
	if attachment == nil {
		return nil, fmt.Errorf("attachment is nil")
	}

	filename := ""
	if attachment.URL != nil {
		parsed, err := url.Parse(attachment.URL.String())
		if err != nil {
			return nil, fmt.Errorf("parse attachment url %q: %w", attachment.URL.String(), err)
		}
		if base := path.Base(parsed.Path); base != "" && base != "." && base != "/" {
			filename = base
		}
	}
	if filename == "" {
		filename = fmt.Sprintf("attachment-%03d%s", index+1, attachmentExtension(attachment.ContentType))
	} else if path.Ext(filename) == "" {
		filename += attachmentExtension(attachment.ContentType)
	}

	targetPath, err := uniqueAttachmentPath(filepath.Join(out, filename))
	if err != nil {
		return nil, err
	}

	return &url.URL{Scheme: "file", Path: targetPath}, nil
}

func attachmentExtension(contentType string) string {
	mediaType := strings.TrimSpace(contentType)
	if parsed, _, err := mime.ParseMediaType(contentType); err == nil {
		mediaType = parsed
	}
	if exts, err := mime.ExtensionsByType(mediaType); err == nil && len(exts) > 0 {
		return exts[0]
	}
	return ".bin"
}

func uniqueAttachmentPath(targetPath string) (string, error) {
	if targetPath == "" {
		return "", fmt.Errorf("attachment path is empty")
	}
	if _, err := os.Stat(targetPath); os.IsNotExist(err) {
		return targetPath, nil
	} else if err != nil {
		return "", fmt.Errorf("stat attachment path %q: %w", targetPath, err)
	}

	ext := filepath.Ext(targetPath)
	base := strings.TrimSuffix(filepath.Base(targetPath), ext)
	dir := filepath.Dir(targetPath)
	for attempt := 2; ; attempt++ {
		candidate := filepath.Join(dir, fmt.Sprintf("%s-%d%s", base, attempt, ext))
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			return candidate, nil
		} else if err != nil {
			return "", fmt.Errorf("stat attachment path %q: %w", candidate, err)
		}
	}
}

func markdownOptsForStdout() []tui.Opt {
	if !term.IsTerminal(int(os.Stdout.Fd())) {
		return nil
	}
	if width, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil && width > 0 {
		return []tui.Opt{tui.SetWidth(width)}
	}
	return nil
}

func writeMarkdown(w io.Writer, widget interface {
	Write(io.Writer, string) (int, error)
}, text string) error {
	if _, err := widget.Write(w, text); err != nil {
		return err
	}
	_, err := io.WriteString(w, "\n")
	return err
}

func newMarkdownStream(w io.Writer, widget interface {
	Write(io.Writer, string) (int, error)
}) *markdownStream {
	return &markdownStream{writer: w, widget: widget, first: true}
}

func (m *markdownStream) Append(chunk string) error {
	if chunk == "" {
		return nil
	}
	m.text.WriteString(chunk)
	flushable, pending := splitMarkdownFlushable(m.text.String())
	if flushable == "" {
		return nil
	}
	m.text.Reset()
	m.text.WriteString(pending)
	if err := m.writeChunk(flushable); err != nil {
		return err
	}
	return nil
}

func (m *markdownStream) Finish(text string) error {
	if text == "" {
		text = m.text.String()
	} else {
		text = m.text.String() + text[len(m.text.String()):]
	}
	text = strings.TrimSpace(text)
	if text == "" {
		if m.first {
			return nil
		}
		_, err := io.WriteString(m.writer, "\n")
		return err
	}
	if err := m.writeChunk(text); err != nil {
		return err
	}
	_, err := io.WriteString(m.writer, "\n")
	return err
}

func (m *markdownStream) writeChunk(text string) error {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	if !m.first {
		if _, err := io.WriteString(m.writer, "\n\n"); err != nil {
			return err
		}
	}
	m.first = false
	_, err := m.widget.Write(m.writer, text)
	return err
}

func splitMarkdownFlushable(text string) (flushable, pending string) {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	if text == "" {
		return "", ""
	}

	lastBoundary := -1
	inFence := false
	index := 0
	for _, line := range strings.SplitAfter(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			inFence = !inFence
		}
		index += len(line)
		if !inFence && (trimmed == "" || strings.HasPrefix(trimmed, "#")) {
			lastBoundary = index
		}
	}

	if inFence || lastBoundary <= 0 || lastBoundary >= len(text) {
		return "", text
	}
	return strings.TrimSpace(text[:lastBoundary]), text[lastBoundary:]
}
