package cmd

import (
	"fmt"

	// Packages
	uuid "github.com/google/uuid"
	otel "github.com/mutablelogic/go-client/pkg/otel"
	httpclient "github.com/mutablelogic/go-llm/pkg/httpclient"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	server "github.com/mutablelogic/go-server"
	types "github.com/mutablelogic/go-server/pkg/types"
	attribute "go.opentelemetry.io/otel/attribute"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type SessionCommands struct {
	CreateSession CreateSessionCommand `cmd:"" name:"session-create" help:"Create a new session." group:"SESSIONS"`
	GetSession    GetSessionCommand    `cmd:"" name:"session" help:"Get a session by ID." group:"SESSIONS"`
	DeleteSession DeleteSessionCommand `cmd:"" name:"session-delete" help:"Delete a session by ID." group:"SESSIONS"`
}

type CreateSessionCommand struct {
	schema.SessionInsert `embed:""`
}

type GetSessionCommand struct {
	ID uuid.UUID `arg:"" name:"id" help:"Session ID."`
}

type DeleteSessionCommand struct {
	ID uuid.UUID `arg:"" name:"id" help:"Session ID."`
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

func (cmd *CreateSessionCommand) Run(ctx server.Cmd) (err error) {
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
	return WithClient(ctx, func(client *httpclient.Client, _ string) error {
		parent, endSpan := otel.StartSpan(ctx.Tracer(), ctx.Context(), "GetSessionCommand",
			attribute.String("id", cmd.ID.String()),
		)
		defer func() { endSpan(err) }()

		session, err := client.GetSession(parent, cmd.ID)
		if err != nil {
			return err
		}

		fmt.Println(session)
		return nil
	})
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
