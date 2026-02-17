package main

import (
	"fmt"
	"strings"

	// Packages
	otel "github.com/mutablelogic/go-client/pkg/otel"
	httpclient "github.com/mutablelogic/go-llm/pkg/httpclient"
	opt "github.com/mutablelogic/go-llm/pkg/opt"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	uitable "github.com/mutablelogic/go-llm/pkg/ui/table"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type SessionCommands struct {
	ListSessions  ListSessionsCommand  `cmd:"" name:"sessions" help:"List sessions." group:"SESSION"`
	GetSession    GetSessionCommand    `cmd:"" name:"session" help:"Get session." group:"SESSION"`
	CreateSession CreateSessionCommand `cmd:"" name:"create-session" help:"Create a new session." group:"SESSION"`
	UpdateSession UpdateSessionCommand `cmd:"" name:"update-session" help:"Update a session." group:"SESSION"`
	DeleteSession DeleteSessionCommand `cmd:"" name:"delete-session" help:"Delete a session." group:"SESSION"`
}

type ListSessionsCommand struct {
	Limit  *uint    `name:"limit" help:"Maximum number of sessions to return" optional:""`
	Offset uint     `name:"offset" help:"Offset for pagination" default:"0"`
	Label  []string `name:"label" help:"Filter by label (key:value)" optional:""`
}

type GetSessionCommand struct {
	ID string `arg:"" name:"id" help:"Session ID (defaults to current session)" optional:""`
}

type CreateSessionCommand struct {
	Model        string `arg:"" name:"model" help:"Model name"`
	Name         string `name:"name" help:"Session name" optional:""`
	Provider     string `name:"provider" help:"Provider name" optional:""`
	SystemPrompt string `name:"system-prompt" help:"System prompt" optional:""`
}

type UpdateSessionCommand struct {
	ID           string   `arg:"" name:"id" help:"Session ID"`
	Name         string   `name:"name" help:"Session name" optional:""`
	Model        string   `name:"model" help:"Model name" optional:""`
	Provider     string   `name:"provider" help:"Provider name" optional:""`
	SystemPrompt string   `name:"system-prompt" help:"System prompt" optional:""`
	Label        []string `name:"label" help:"Set label (key=value)" optional:""`
}

type DeleteSessionCommand struct {
	ID string `arg:"" name:"id" help:"Session ID"`
}

///////////////////////////////////////////////////////////////////////////////
// COMMANDS

func (cmd *ListSessionsCommand) Run(ctx *Globals) (err error) {
	client, err := ctx.Client()
	if err != nil {
		return err
	}

	// OTEL
	parent, endSpan := otel.StartSpan(ctx.tracer, ctx.ctx, "ListSessionsCommand")
	defer func() { endSpan(err) }()

	// Build options
	opts := []opt.Opt{}
	if cmd.Limit != nil {
		opts = append(opts, httpclient.WithLimit(cmd.Limit))
	}
	if cmd.Offset > 0 {
		opts = append(opts, httpclient.WithOffset(cmd.Offset))
	}
	for _, l := range cmd.Label {
		k, v, ok := strings.Cut(l, ":")
		if !ok {
			// Also accept key=value format
			k, v, ok = strings.Cut(l, "=")
		}
		if !ok || k == "" {
			return fmt.Errorf("invalid label filter %q (use key:value or key=value)", l)
		}
		opts = append(opts, httpclient.WithLabel(k, v))
	}

	// List sessions
	response, err := client.ListSessions(parent, opts...)
	if err != nil {
		return err
	}

	// Print
	if ctx.Debug {
		fmt.Println(response)
	} else {
		if len(response.Body) > 0 {
			fmt.Println(uitable.Render(schema.SessionTable{
				Sessions:       response.Body,
				CurrentSession: ctx.defaults.GetString("session"),
			}))
		}
		fmt.Println(TableSummary(len(response.Body), int(response.Offset), int(response.Count)))
	}
	return nil
}

func (cmd *GetSessionCommand) Run(ctx *Globals) (err error) {
	client, err := ctx.Client()
	if err != nil {
		return err
	}

	// Use current session if no ID provided
	id := cmd.ID
	if id == "" {
		id = ctx.defaults.GetString("session")
	}
	if id == "" {
		return fmt.Errorf("no session ID provided and no current session set")
	}

	// OTEL
	parent, endSpan := otel.StartSpan(ctx.tracer, ctx.ctx, "GetSessionCommand")
	defer func() { endSpan(err) }()

	// Get session
	session, err := client.GetSession(parent, id)
	if err != nil {
		// If the default session is stale, clear it
		if cmd.ID == "" && isNotFound(err) {
			ctx.defaults.Set("session", "")
			return fmt.Errorf("no session ID provided and no current session set")
		}
		return err
	}

	// Persist as current default session
	if err := ctx.defaults.Set("session", session.ID); err != nil {
		return err
	}

	// Print
	fmt.Println(session)
	return nil
}

func (cmd *CreateSessionCommand) Run(ctx *Globals) (err error) {
	client, err := ctx.Client()
	if err != nil {
		return err
	}

	// OTEL
	parent, endSpan := otel.StartSpan(ctx.tracer, ctx.ctx, "CreateSessionCommand")
	defer func() { endSpan(err) }()

	// Create session
	session, err := client.CreateSession(parent, schema.SessionMeta{
		Name: cmd.Name,
		GeneratorMeta: schema.GeneratorMeta{
			Provider:     cmd.Provider,
			Model:        cmd.Model,
			SystemPrompt: cmd.SystemPrompt,
		},
	})
	if err != nil {
		return err
	}

	// Set as default session
	if err := ctx.defaults.Set("session", session.ID); err != nil {
		return err
	}

	// Print
	fmt.Println(session)
	return nil
}

func (cmd *DeleteSessionCommand) Run(ctx *Globals) (err error) {
	client, err := ctx.Client()
	if err != nil {
		return err
	}

	// OTEL
	parent, endSpan := otel.StartSpan(ctx.tracer, ctx.ctx, "DeleteSessionCommand")
	defer func() { endSpan(err) }()

	// Delete session
	if err := client.DeleteSession(parent, cmd.ID); err != nil {
		return err
	}

	// Print
	fmt.Printf("Deleted session %s\n", cmd.ID)
	return nil
}

func (cmd *UpdateSessionCommand) Run(ctx *Globals) (err error) {
	client, err := ctx.Client()
	if err != nil {
		return err
	}

	// OTEL
	parent, endSpan := otel.StartSpan(ctx.tracer, ctx.ctx, "UpdateSessionCommand")
	defer func() { endSpan(err) }()

	// Parse labels
	labels := make(map[string]string)
	for _, l := range cmd.Label {
		k, v, ok := strings.Cut(l, "=")
		if !ok || k == "" {
			return fmt.Errorf("invalid label %q (use key=value)", l)
		}
		labels[k] = v
	}

	// Update session
	meta := schema.SessionMeta{
		Name: cmd.Name,
		GeneratorMeta: schema.GeneratorMeta{
			Provider:     cmd.Provider,
			Model:        cmd.Model,
			SystemPrompt: cmd.SystemPrompt,
		},
	}
	if len(labels) > 0 {
		meta.Labels = labels
	}
	session, err := client.UpdateSession(parent, cmd.ID, meta)
	if err != nil {
		return err
	}

	// Print
	fmt.Println(session)
	return nil
}
