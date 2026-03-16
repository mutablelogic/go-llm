package main

import (
	"encoding/json"
	"fmt"
	"mime"
	"os"
	"path/filepath"

	// Packages
	otel "github.com/mutablelogic/go-client/pkg/otel"
	httpclient "github.com/mutablelogic/go-llm/pkg/httpclient"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	server "github.com/mutablelogic/go-server"
	gocmd "github.com/mutablelogic/go-server/pkg/cmd"
	types "github.com/mutablelogic/go-server/pkg/types"
	"go.opentelemetry.io/otel/attribute"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type GenerateCommands struct {
	Ask       AskCommand       `cmd:"" name:"ask" help:"Send a stateless text request to a model." group:"GENERATE"`
	Chat      ChatCommand      `cmd:"" name:"chat" help:"Send a message within a session (creates one if needed)." group:"GENERATE"`
	Embedding EmbeddingCommand `cmd:"" name:"embedding" help:"Generate embedding vectors from text." group:"GENERATE"`
}

type AskCommand struct {
	schema.AskRequest
	File []string `help:"Path or glob pattern for files to attach (may be repeated)" optional:""`
	URL  string   `help:"URL to attach as a reference" optional:""`
}

///////////////////////////////////////////////////////////////////////////////
// COMMANDS

func (cmd *AskCommand) Run(ctx server.Cmd) (err error) {
	// Load defaults for model and provider when not explicitly set
	if cmd.AskRequest.Model == "" {
		cmd.AskRequest.Model = ctx.GetString("model")
	}
	if cmd.AskRequest.Provider == "" {
		cmd.AskRequest.Provider = ctx.GetString("provider")
	}
	if cmd.AskRequest.Model == "" {
		return fmt.Errorf("model is required (set with --model or store a default)")
	}

	// Store model and provider as defaults
	if err := ctx.Set("model", cmd.AskRequest.Model); err != nil {
		return err
	}
	if cmd.AskRequest.Provider != "" {
		if err := ctx.Set("provider", cmd.AskRequest.Provider); err != nil {
			return err
		}
	}

	// When format is set, hint the model to reply in JSON
	if len(cmd.AskRequest.Format) > 0 {
		cmd.AskRequest.SystemPrompt = "Respond with valid JSON only. Do not include any other text or formatting.\n" + cmd.AskRequest.SystemPrompt
	}

	client, err := clientFor(ctx)
	if err != nil {
		return err
	}

	// OTEL
	parent, endSpan := otel.StartSpan(ctx.Tracer(), ctx.Context(), "AskCommand",
		attribute.String("request", types.Stringify(cmd)),
	)
	defer func() { endSpan(err) }()

	// Build options
	var opts []httpclient.AskOpt
	for _, pattern := range cmd.File {
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
			defer f.Close()
			opts = append(opts, httpclient.WithFile(f.Name(), f))
		}
	}
	if cmd.URL != "" {
		opts = append(opts, httpclient.WithURL(cmd.URL))
	}

	// Send request
	response, err := client.Ask(parent, cmd.AskRequest, opts...)
	if err != nil {
		return err
	}

	// Print
	if ctx.IsDebug() {
		fmt.Println(response)
	} else {
		// Collect text, thinking, and attachments from content blocks
		var text, thinking string
		for _, block := range response.Content {
			if block.Text != nil {
				text += *block.Text
			}
			if block.Thinking != nil {
				thinking += *block.Thinking
			}
			if block.Attachment != nil && len(block.Attachment.Data) > 0 {
				if err := saveAttachment(block.Attachment); err != nil {
					fmt.Fprintf(os.Stderr, "warning: could not save attachment: %v\n", err)
				}
			}
		}

		// If format was set, try to pretty-print as indented JSON
		if len(cmd.AskRequest.Format) > 0 {
			var raw json.RawMessage
			if err := json.Unmarshal([]byte(text), &raw); err == nil {
				if indented, err := json.MarshalIndent(raw, "", "  "); err == nil {
					fmt.Println(string(indented))
					return nil
				}
			}
		}

		// Print thinking block if present
		if thinking != "" {
			label := "thinking"
			if gocmd.IsTerminal() {
				label = "\033[2m" + label + "\033[0m" // dim
				thinking = "\033[2m" + thinking + "\033[0m"
			}
			fmt.Println(label + ": " + thinking)
			fmt.Println()
		}

		// Prepend role
		role := response.Role
		if role != "" {
			if gocmd.IsTerminal() {
				role = "\033[1m" + role + "\033[0m"
			}
			text = role + ": " + text
		}
		fmt.Println(text)
	}
	return nil
}

// saveAttachment writes attachment data to a temp file, prints the path, and
// attempts to open the file in the system viewer.
func saveAttachment(a *schema.Attachment) error {
	// Derive a file extension from the MIME type
	ext := ""
	if a.ContentType != "" {
		if exts, err := mime.ExtensionsByType(a.ContentType); err == nil && len(exts) > 0 {
			ext = exts[len(exts)-1] // prefer the last (most canonical) extension
		}
	}
	f, err := os.CreateTemp("", "llm-*"+ext)
	if err != nil {
		return err
	}
	if _, err = f.Write(a.Data); err != nil {
		f.Close()
		return err
	}
	name := f.Name()
	// Close before opening in the browser so the file is fully flushed.
	if err = f.Close(); err != nil {
		return err
	}
	// Print to stderr to avoid corrupting stdout (e.g. when --format is used).
	fmt.Fprintln(os.Stderr, "attachment:", name)
	_ = openBrowser(name)
	return nil
}
