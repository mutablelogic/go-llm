package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"
	"uuid"

	// Packages
	providerschema "github.com/mutablelogic/go-llm/provider/schema"
	httpclient "github.com/mutablelogic/go-llm/registry/httpclient"
	schema "github.com/mutablelogic/go-llm/registry/schema"
	server "github.com/mutablelogic/go-server"
	tui "github.com/mutablelogic/go-server/pkg/tui"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type SessionCommands struct {
	SessionList   SessionListCmd   `cmd:"" name:"sessions" help:"List the sessions, most recently modified first." group:"LLM SESSIONS"`
	SessionGet    SessionGetCmd    `cmd:"" name:"session" help:"Get a session, and make it the default." group:"LLM SESSIONS"`
	SessionMsgs   SessionMsgsCmd   `cmd:"" name:"session-messages" help:"List a session's messages, oldest first." group:"LLM SESSIONS"`
	SessionCreate SessionCreateCmd `cmd:"" name:"session-create" help:"Create a session, and make it the default." group:"LLM SESSIONS"`
	SessionUpdate SessionUpdateCmd `cmd:"" name:"session-update" help:"Change a session's settings. An empty value clears one, and --opt key=null removes an option." group:"LLM SESSIONS"`
	SessionDelete SessionDeleteCmd `cmd:"" name:"session-delete" help:"Delete a session." group:"LLM SESSIONS"`
}

type SessionListCmd struct {
	Offset uint  `name:"offset" help:"Sessions to skip." optional:""`
	Limit  *uint `name:"limit" help:"Most sessions to list." optional:""`
}

// SessionGetCmd gets a session, which is the default session when there's no
// identifier, and is then the default
type SessionGetCmd struct {
	ID uuid.UUID `arg:"" name:"id" optional:"" help:"Session identifier, which is a UUID, or the default session."`
}

// SessionMsgsCmd lists the messages of a session, which is the default
// session when there's no identifier, and is then the default
type SessionMsgsCmd struct {
	ID     uuid.UUID `arg:"" name:"id" optional:"" help:"Session identifier, which is a UUID, or the default session."`
	Role   string    `name:"role" help:"List only the messages of this role, such as user, assistant or tool." optional:""`
	Offset uint      `name:"offset" help:"Messages to skip." optional:""`
	Limit  *uint     `name:"limit" help:"Most messages to list." optional:""`
}

// SessionCreateCmd creates a session with the settings which are given, and
// the default model, when no model is given, which is then the default
// session
type SessionCreateCmd struct {
	schema.SessionMeta
}

// SessionUpdateCmd changes the settings of a session, which is the default
// session when there's no identifier, and is then the default, which are
// given, and keeps the rest: an empty model, system prompt or tools, such as --tools=,
// clears them, --schema=null clears the schema, and an option which is null,
// such as --opt thinking=null, is removed
type SessionUpdateCmd struct {
	ID uuid.UUID `arg:"" name:"id" optional:"" help:"Session identifier, which is a UUID, or the default session."`
	schema.SessionMeta
}

// SessionDeleteCmd deletes a session, which isn't the default, when it's
// deleted
type SessionDeleteCmd struct {
	ID uuid.UUID `arg:"" name:"id" help:"Session identifier, which is a UUID."`
}

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

// defaultSessionKey is the key of the default session, which is the session
// last created, got or updated
const defaultSessionKey = "llm.session"

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
	id, err := sessionID(ctx, cmd.ID)
	if err != nil {
		return err
	}
	return withClient(ctx, "llm.SessionGetCmd", func(reqctx context.Context, client *httpclient.Client) error {
		session, err := client.GetSession(reqctx, id)
		if err != nil {
			return err
		}
		fmt.Println(session)
		return ctx.Set(defaultSessionKey, session.ID.String())
	})
}

func (cmd *SessionMsgsCmd) Run(ctx server.Cmd) error {
	debug := ctx.IsDebug()
	id, err := sessionID(ctx, cmd.ID)
	if err != nil {
		return err
	}
	return withClient(ctx, "llm.SessionMsgsCmd", func(reqctx context.Context, client *httpclient.Client) error {
		list, err := client.ListSessionMessages(reqctx, id, schema.MessageListRequest{Role: cmd.Role, Offset: cmd.Offset, Limit: cmd.Limit})
		if err != nil {
			return err
		}
		if err := ctx.Set(defaultSessionKey, id.String()); err != nil {
			return err
		}

		// With debugging, the response
		if debug {
			fmt.Println(list)
			return nil
		}

		// Otherwise each message, and a summary
		for _, message := range list.Body {
			printMessage(message)
		}
		summary := tui.TableSummary("messages", list.Count, uint(len(list.Body)), uint64(cmd.Offset), nil)
		_, err = summary.Write(os.Stdout)
		return err
	})
}

func (cmd *SessionCreateCmd) Run(ctx server.Cmd) error {
	meta := createMeta(ctx, cmd.SessionMeta)
	return withClient(ctx, "llm.SessionCreateCmd", func(reqctx context.Context, client *httpclient.Client) error {
		session, err := client.CreateSession(reqctx, meta)
		if err != nil {
			return err
		}
		fmt.Println(session)
		return ctx.Set(defaultSessionKey, session.ID.String())
	})
}

func (cmd *SessionUpdateCmd) Run(ctx server.Cmd) error {
	id, err := sessionID(ctx, cmd.ID)
	if err != nil {
		return err
	}
	return withClient(ctx, "llm.SessionUpdateCmd", func(reqctx context.Context, client *httpclient.Client) error {
		meta := cmd.SessionMeta
		meta.Opt = quoteOpts(meta.Opt)
		session, err := client.UpdateSession(reqctx, id, meta)
		if err != nil {
			return err
		}
		fmt.Println(session)
		return ctx.Set(defaultSessionKey, session.ID.String())
	})
}

func (cmd *SessionDeleteCmd) Run(ctx server.Cmd) error {
	return withClient(ctx, "llm.SessionDeleteCmd", func(reqctx context.Context, client *httpclient.Client) error {
		session, err := client.DeleteSession(reqctx, cmd.ID)
		if err != nil {
			return err
		}
		fmt.Println(session)
		if ctx.GetString(defaultSessionKey) == session.ID.String() {
			return ctx.Set(defaultSessionKey, nil)
		}
		return nil
	})
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// sessionID returns the identifier, or the default session's, when it's
// the nil UUID, as it wasn't given. Returns an error when there's no default
// session, or it isn't a UUID.
func sessionID(defaults interface{ GetString(string) string }, id uuid.UUID) (uuid.UUID, error) {
	if id != uuid.Nil() {
		return id, nil
	}
	value := strings.TrimSpace(defaults.GetString(defaultSessionKey))
	if value == "" {
		return uuid.Nil(), fmt.Errorf("missing session: give its identifier, or create one with session-create, and it's used until another is")
	}
	id, err := uuid.Parse(value)
	if err != nil {
		return uuid.Nil(), fmt.Errorf("the default session %q isn't a UUID: %w", value, err)
	}
	return id, nil
}

// createMeta returns the settings of a session to create, with the default
// model, when no model is given, so --model= creates one without a model,
// and options which aren't JSON as JSON strings
func createMeta(defaults interface{ GetString(string) string }, meta schema.SessionMeta) schema.SessionMeta {
	meta.Opt = quoteOpts(meta.Opt)
	if meta.Model == nil {
		if model := strings.TrimSpace(defaults.GetString(defaultModelKey)); model != "" {
			meta.Model = types.Ptr(model)
		}
	}
	return meta
}

// printMessage prints a message: its role, and its text, thinking, the tools
// it called, and their results, a line each
func printMessage(message providerschema.Message) {
	fmt.Printf("[%s]", message.Role())
	if message.Tokens > 0 {
		fmt.Printf(" %d tokens", message.Tokens)
	}
	if message.Result != "" {
		fmt.Printf(" %s", message.Result)
	}
	fmt.Println()
	for _, block := range message.Content {
		switch {
		case block.Text != nil && strings.TrimSpace(*block.Text) != "":
			fmt.Println(strings.TrimSpace(*block.Text))
		case block.Thinking != nil && strings.TrimSpace(*block.Thinking) != "":
			fmt.Println("(thinking)", strings.TrimSpace(*block.Thinking))
		case block.ToolCall != nil:
			fmt.Printf("→ %s(%s)\n", block.ToolCall.Name, string(block.ToolCall.Input))
		case block.ToolResult != nil:
			fmt.Printf("← %s: %s\n", block.ToolResult.Name, string(block.ToolResult.Content))
		case block.Resource != nil:
			fmt.Printf("(attachment %s, %d bytes)\n", block.Resource.ContentType, len(block.Resource.Data))
		}
	}
	fmt.Println()
}
