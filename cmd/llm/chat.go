package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	// Packages
	otel "github.com/mutablelogic/go-client/pkg/otel"
	httpclient "github.com/mutablelogic/go-llm/pkg/httpclient"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type ChatCommand struct {
	Text         string   `arg:"" help:"User input text"`
	Model        string   `name:"model" help:"Model name (used when creating a new session)" optional:""`
	Session      string   `name:"session" help:"Session ID (overrides stored default)" optional:""`
	SystemPrompt string   `name:"system-prompt" help:"System prompt (used when creating a new session)" optional:""`
	File         []string `help:"Path or glob pattern for files to attach (may be repeated)" optional:""`
	URL          string   `help:"URL to attach as a reference" optional:""`
	Tool         []string `name:"tool" help:"Tool names to include (may be repeated; empty means all)" optional:""`
	New          bool     `name:"new" help:"Force creation of a new session" optional:""`
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

	// Determine session ID: explicit flag > stored default
	sessionID := cmd.Session
	if sessionID == "" && !cmd.New {
		sessionID = ctx.defaults.GetString("session")
	}

	// Verify the session still exists; clear it if not
	if sessionID != "" {
		if _, err := client.GetSession(parent, sessionID); err != nil {
			sessionID = ""
		}
	}

	// If we still have no session, create one
	if sessionID == "" {
		model := cmd.Model
		if model == "" {
			model = ctx.defaults.GetString("model")
		}
		if model == "" {
			return fmt.Errorf("model is required to create a session (set with --model or store a default)")
		}

		provider := ctx.defaults.GetString("provider")

		session, err := client.CreateSession(parent, schema.SessionMeta{
			GeneratorMeta: schema.GeneratorMeta{
				Provider:     provider,
				Model:        model,
				SystemPrompt: cmd.SystemPrompt,
			},
		})
		if err != nil {
			return fmt.Errorf("creating session: %w", err)
		}
		sessionID = session.ID
	} else {
		// If reusing an existing session but parameters were changed, update it
		meta := schema.SessionMeta{
			GeneratorMeta: schema.GeneratorMeta{
				Model:        cmd.Model,
				SystemPrompt: cmd.SystemPrompt,
			},
		}
		if meta.Model != "" || meta.SystemPrompt != "" {
			if _, err := client.UpdateSession(parent, sessionID, meta); err != nil {
				return fmt.Errorf("updating session: %w", err)
			}
		}
	}

	// Persist the session ID as the current default
	if err := ctx.defaults.Set("session", sessionID); err != nil {
		return err
	}

	// Build file attachment options
	var opts []httpclient.ChatOpt
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
			opts = append(opts, httpclient.WithChatFile(f.Name(), f))
		}
	}
	if cmd.URL != "" {
		opts = append(opts, httpclient.WithChatURL(cmd.URL))
	}

	// Build request
	req := schema.ChatRequest{
		Session: sessionID,
		Text:    cmd.Text,
		Tools:   cmd.Tool,
	}

	// Send request
	response, err := client.Chat(parent, req, opts...)
	if err != nil {
		return err
	}

	// Print
	if ctx.Debug {
		fmt.Println(response)
	} else {
		// Collect text and thinking from content blocks
		var text, thinking string
		for _, block := range response.Content {
			if block.Text != nil {
				text += *block.Text
			}
			if block.Thinking != nil {
				thinking += *block.Thinking
			}
		}

		// If the response contains JSON, try to pretty-print it
		var raw json.RawMessage
		if json.Unmarshal([]byte(text), &raw) == nil {
			if indented, err := json.MarshalIndent(raw, "", "  "); err == nil {
				fmt.Println(string(indented))
				return nil
			}
		}

		// Print thinking block if present
		if thinking != "" {
			label := "thinking"
			if isTerminal(os.Stdout) {
				label = "\033[2m" + label + "\033[0m"
				thinking = "\033[2m" + thinking + "\033[0m"
			}
			fmt.Println(label + ": " + thinking)
			fmt.Println()
		}

		// Prepend role
		role := response.Role
		if role != "" {
			if isTerminal(os.Stdout) {
				role = "\033[1m" + role + "\033[0m"
			}
			text = role + ": " + text
		}
		fmt.Println(text)
	}
	return nil
}
