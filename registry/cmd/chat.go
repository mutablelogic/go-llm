package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"
	"uuid"

	// Packages
	providerschema "github.com/mutablelogic/go-llm/provider/schema"
	chattui "github.com/mutablelogic/go-llm/registry/cmd/tui"
	httpclient "github.com/mutablelogic/go-llm/registry/httpclient"
	schema "github.com/mutablelogic/go-llm/registry/schema"
	server "github.com/mutablelogic/go-server"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// ChatCmd asks a session's model a question, in the session, which keeps the
// question, and the reply, or, without a question, runs an interface in the
// terminal to chat in the session
type ChatCmd struct {
	Text    []string  `arg:"" name:"text" help:"The question to ask, which can be more than one argument, or, without one, chat in the terminal." optional:""`
	Files   []string  `name:"file" short:"f" sep:"none" help:"A file the question is about, which can be given more than once. Text, such as code or subtitles, is sent as text, with its name, and anything else, such as an image or a PDF, as an attachment, which the model must accept." optional:""`
	Session uuid.UUID `name:"session" help:"Session identifier, which is a UUID, or the default session, which this is then." optional:""`
	Stream  bool      `name:"stream" negatable:"" default:"true" help:"Print the reply as it's generated, or with --no-stream, once it's complete, rendered as markdown on a terminal."`
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Run asks the session's model the question, which runs the tools it calls,
// and prints its reply, as it's generated, or once it's complete, and why it
// stopped, when it didn't finish, and keeps the session as the default
func (cmd *ChatCmd) Run(ctx server.Cmd) error {
	debug := ctx.IsDebug()
	width := ctx.IsTerm()

	// The session, and the question
	id, err := sessionID(ctx, cmd.Session)
	if err != nil {
		return err
	}
	text := strings.TrimSpace(strings.Join(cmd.Text, " "))
	if text == "" && len(cmd.Files) == 0 && width > 0 {
		if err := withClient(ctx, "llm.ChatCmd", func(reqctx context.Context, client *httpclient.Client) error {
			return chattui.Chat(reqctx, client, id)
		}); err != nil {
			return err
		}
		return ctx.Set(defaultSessionKey, id.String())
	} else if text == "" {
		return fmt.Errorf("missing question")
	}
	message, err := askMessage(text, cmd.Files)
	if err != nil {
		return err
	}

	return withClient(ctx, "llm.ChatCmd", func(reqctx context.Context, client *httpclient.Client) error {
		printer := newStreamPrinter(debug || !cmd.Stream, width)
		response, err := client.Chat(reqctx, id, schema.ChatRequest{
			Messages: []providerschema.MessageMeta{message.MessageMeta},
		}, printer.Stream())
		printer.End()
		if err != nil {
			return err
		}

		// The session is the default once its model has replied
		if err := ctx.Set(defaultSessionKey, response.ID.String()); err != nil {
			return err
		}

		// With debugging, the response
		if debug {
			fmt.Println(response)
			return nil
		}

		// Otherwise the reply's text, unless it was streamed, and why it
		// stopped, when it didn't finish
		if len(response.Messages) == 0 {
			return nil
		}
		reply := response.Messages[len(response.Messages)-1]
		if text := reply.Text(); text != "" && !printer.Streamed() {
			if err := printText(width, text); err != nil {
				return err
			}
		}
		if reply.Result != "" && reply.Result != providerschema.ResultStop {
			fmt.Fprintln(os.Stderr, "The reply stopped:", reply.Result)
		}
		return nil
	})
}
