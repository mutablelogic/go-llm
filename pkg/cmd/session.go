package cmd

import (
	"fmt"
	"os"

	// Packages
	uuid "github.com/google/uuid"
	otel "github.com/mutablelogic/go-client/pkg/otel"
	httpclient "github.com/mutablelogic/go-llm/pkg/httpclient"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	tui "github.com/mutablelogic/go-llm/pkg/tui"
	server "github.com/mutablelogic/go-server"
	types "github.com/mutablelogic/go-server/pkg/types"
	attribute "go.opentelemetry.io/otel/attribute"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type SessionCommands struct {
	ListSessions  ListSessionsCommand  `cmd:"" name:"sessions" help:"List sessions." group:"SESSIONS"`
	CreateSession CreateSessionCommand `cmd:"" name:"session-create" help:"Create a new session." group:"SESSIONS"`
	GetSession    GetSessionCommand    `cmd:"" name:"session" help:"Get a session by ID or the stored current session." group:"SESSIONS"`
	UpdateSession UpdateSessionCommand `cmd:"" name:"session-update" help:"Update session metadata." group:"SESSIONS"`
	DeleteSession DeleteSessionCommand `cmd:"" name:"session-delete" help:"Delete a session by ID." group:"SESSIONS"`
}

type ListSessionsCommand struct {
	schema.SessionListRequest `embed:""`
}

type CreateSessionCommand struct {
	schema.SessionInsert `embed:""`
}

type GetSessionCommand struct {
	ID uuid.UUID `arg:"" name:"id" help:"Session ID (defaults to the stored current session)." optional:""`
}

type DeleteSessionCommand struct {
	ID uuid.UUID `arg:"" name:"id" help:"Session ID."`
}

type UpdateSessionCommand struct {
	ID                 uuid.UUID `arg:"" name:"id" help:"Session ID."`
	schema.SessionMeta `embed:""`
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

func (cmd *ListSessionsCommand) Run(ctx server.Cmd) (err error) {
	return WithClient(ctx, func(client *httpclient.Client, _ string) error {
		parent, endSpan := otel.StartSpan(ctx.Tracer(), ctx.Context(), "ListSessionsCommand",
			attribute.String("request", types.Stringify(cmd.SessionListRequest)),
		)
		defer func() { endSpan(err) }()

		sessions, err := client.ListSessions(parent, cmd.SessionListRequest)
		if err != nil {
			return err
		}

		if ctx.IsDebug() {
			fmt.Println(sessions)
			return nil
		}

		_, err = tui.TableFor[*schema.Session]().Write(os.Stdout, sessions.Body...)
		return err
	})
}

func (cmd *CreateSessionCommand) Run(ctx server.Cmd) (err error) {
	// Only load defaults and require a model when no parent is set.
	// With a parent, the model/provider are inherited server-side.
	if cmd.Parent == uuid.Nil {
		if cmd.Model == nil {
			if s := ctx.GetString("model"); s != "" {
				cmd.Model = types.Ptr(s)
			}
		}
		if cmd.Provider == nil {
			if s := ctx.GetString("provider"); s != "" {
				cmd.Provider = types.Ptr(s)
			}
		}
		if cmd.Model == nil {
			return fmt.Errorf("model is required (set with --model or store a default)")
		}
	}

	return WithClient(ctx, func(client *httpclient.Client, _ string) error {
		parent, endSpan := otel.StartSpan(ctx.Tracer(), ctx.Context(), "CreateSessionCommand",
			attribute.String("request", types.Stringify(cmd.SessionInsert)),
		)
		defer func() { endSpan(err) }()

		session, err := client.CreateSession(parent, cmd.SessionInsert)
		if err != nil {
			return err
		}

		fmt.Println(session)
		return nil
	})
}

func (cmd *GetSessionCommand) Run(ctx server.Cmd) (err error) {
	id, err := resolveSessionID(cmd.ID, ctx.GetString("session"))
	if err != nil {
		return err
	}

	return WithClient(ctx, func(client *httpclient.Client, _ string) error {
		parent, endSpan := otel.StartSpan(ctx.Tracer(), ctx.Context(), "GetSessionCommand",
			attribute.String("id", id.String()),
		)
		defer func() { endSpan(err) }()

		session, err := client.GetSession(parent, id)
		if err != nil {
			return err
		}

		fmt.Println(session)
		return nil
	})
}

func resolveSessionID(id uuid.UUID, stored string) (uuid.UUID, error) {
	if id != uuid.Nil {
		return id, nil
	}
	if stored == "" {
		return uuid.Nil, fmt.Errorf("session is required (pass an id or store a default session)")
	}
	parsed, err := uuid.Parse(stored)
	if err != nil {
		return uuid.Nil, fmt.Errorf("invalid stored session %q: %w", stored, err)
	}
	return parsed, nil
}

func (cmd *DeleteSessionCommand) Run(ctx server.Cmd) (err error) {
	return WithClient(ctx, func(client *httpclient.Client, _ string) error {
		parent, endSpan := otel.StartSpan(ctx.Tracer(), ctx.Context(), "DeleteSessionCommand",
			attribute.String("id", cmd.ID.String()),
		)
		defer func() { endSpan(err) }()

		session, err := client.DeleteSession(parent, cmd.ID)
		if err != nil {
			return err
		}

		fmt.Println(session)
		return nil
	})
}

func (cmd *UpdateSessionCommand) Run(ctx server.Cmd) (err error) {
	return WithClient(ctx, func(client *httpclient.Client, _ string) error {
		parent, endSpan := otel.StartSpan(ctx.Tracer(), ctx.Context(), "UpdateSessionCommand",
			attribute.String("id", cmd.ID.String()),
			attribute.String("meta", types.Stringify(cmd.SessionMeta)),
		)
		defer func() { endSpan(err) }()

		session, err := client.UpdateSession(parent, cmd.ID, cmd.SessionMeta)
		if err != nil {
			return err
		}

		fmt.Println(session)
		return nil
	})
}
