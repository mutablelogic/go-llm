package cmd

import (
	"context"
	"fmt"
	"os"
	"uuid"

	// Packages
	httpclient "github.com/mutablelogic/go-llm/registry/httpclient"
	schema "github.com/mutablelogic/go-llm/registry/schema"
	server "github.com/mutablelogic/go-server"
	tui "github.com/mutablelogic/go-server/pkg/tui"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type SessionCommands struct {
	SessionList   SessionListCmd   `cmd:"" name:"sessions" help:"List the sessions, most recently modified first." group:"LLM SESSIONS"`
	SessionGet    SessionGetCmd    `cmd:"" name:"session" help:"Get a session, with its messages." group:"LLM SESSIONS"`
	SessionCreate SessionCreateCmd `cmd:"" name:"session-create" help:"Create a session, with how its model is asked." group:"LLM SESSIONS"`
	SessionUpdate SessionUpdateCmd `cmd:"" name:"session-update" help:"Change a session's settings: those which are given are changed, and the rest kept. An empty value, such as --tools=, clears one, --schema=null clears the schema, and --opt key=null removes an option." group:"LLM SESSIONS"`
	SessionDelete SessionDeleteCmd `cmd:"" name:"session-delete" help:"Delete a session." group:"LLM SESSIONS"`
}

type SessionListCmd struct {
	Offset uint  `name:"offset" help:"Sessions to skip." optional:""`
	Limit  *uint `name:"limit" help:"Most sessions to list." optional:""`
}

type SessionGetCmd struct {
	ID uuid.UUID `arg:"" name:"id" help:"Session identifier, which is a UUID."`
}

// SessionCreateCmd creates a session with the settings which are given
type SessionCreateCmd struct {
	schema.SessionMeta
}

// SessionUpdateCmd changes the settings of a session which are given, and
// keeps the rest: an empty model, system prompt or tools, such as --tools=,
// clears them, --schema=null clears the schema, and an option which is null,
// such as --opt thinking=null, is removed
type SessionUpdateCmd struct {
	ID uuid.UUID `arg:"" name:"id" help:"Session identifier, which is a UUID."`
	schema.SessionMeta
}

type SessionDeleteCmd struct {
	ID uuid.UUID `arg:"" name:"id" help:"Session identifier, which is a UUID."`
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

func (cmd *SessionListCmd) Run(ctx server.Cmd) error {
	width := ctx.IsTerm()
	debug := ctx.IsDebug()
	return withClient(ctx, "llm.SessionListCmd", func(ctx context.Context, client *httpclient.Client) error {
		sessions, err := client.ListSessions(ctx, schema.SessionListRequest{Offset: cmd.Offset, Limit: cmd.Limit})
		if err != nil {
			return err
		}

		// With debugging, the response
		if debug {
			fmt.Println(sessions)
			return nil
		}

		// Otherwise a table of the sessions, and a summary
		table := tui.TableFor[schema.Session](tui.SetWidth(width))
		if _, err := table.Write(os.Stdout, sessions.Body...); err != nil {
			return err
		}
		summary := tui.TableSummary("sessions", sessions.Count, uint(len(sessions.Body)), uint64(cmd.Offset), nil)
		_, err = summary.Write(os.Stdout)
		return err
	})
}

func (cmd *SessionGetCmd) Run(ctx server.Cmd) error {
	return withClient(ctx, "llm.SessionGetCmd", func(ctx context.Context, client *httpclient.Client) error {
		session, err := client.GetSession(ctx, cmd.ID)
		if err != nil {
			return err
		}
		fmt.Println(session)
		return nil
	})
}

func (cmd *SessionCreateCmd) Run(ctx server.Cmd) error {
	return withClient(ctx, "llm.SessionCreateCmd", func(ctx context.Context, client *httpclient.Client) error {
		session, err := client.CreateSession(ctx, cmd.SessionMeta)
		if err != nil {
			return err
		}
		fmt.Println(session)
		return nil
	})
}

func (cmd *SessionUpdateCmd) Run(ctx server.Cmd) error {
	return withClient(ctx, "llm.SessionUpdateCmd", func(ctx context.Context, client *httpclient.Client) error {
		session, err := client.UpdateSession(ctx, cmd.ID, cmd.SessionMeta)
		if err != nil {
			return err
		}
		fmt.Println(session)
		return nil
	})
}

func (cmd *SessionDeleteCmd) Run(ctx server.Cmd) error {
	return withClient(ctx, "llm.SessionDeleteCmd", func(ctx context.Context, client *httpclient.Client) error {
		session, err := client.DeleteSession(ctx, cmd.ID)
		if err != nil {
			return err
		}
		fmt.Println(session)
		return nil
	})
}
