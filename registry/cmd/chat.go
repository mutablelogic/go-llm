package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"uuid"

	// Packages
	eventclient "github.com/mutablelogic/go-llm/broadcaster/httpclient"
	eventschema "github.com/mutablelogic/go-llm/broadcaster/schema"
	providerschema "github.com/mutablelogic/go-llm/provider/schema"
	chattui "github.com/mutablelogic/go-llm/registry/cmd/tui"
	httpclient "github.com/mutablelogic/go-llm/registry/httpclient"
	schema "github.com/mutablelogic/go-llm/registry/schema"
	toolclient "github.com/mutablelogic/go-llm/toolkit/httpclient"
	toolschema "github.com/mutablelogic/go-llm/toolkit/schema"
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

// chatClient is the client of the registry's API, of its events, and of
// the toolkit's, which the interface chats with
type chatClient struct {
	*httpclient.Client
	events *eventclient.Client
	tools  *toolclient.Client
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Run asks the session's model the question, which runs the tools it calls,
// and prints its reply, as it's generated, or once it's complete, or as
// JSON, when the session has a schema, and why it stopped, when it didn't
// finish, and keeps the session as the default
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
			events, err := newEventClient(ctx)
			if err != nil {
				return err
			}
			tools, err := newToolClient(ctx)
			if err != nil {
				return err
			}
			return chattui.Chat(reqctx, chatClient{client, events, tools}, id, debug)
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
		// A reply with a schema is JSON, which is made once the model has
		// replied, so isn't streamed
		session, err := client.GetSession(reqctx, id)
		if err != nil {
			return err
		}
		hasSchema := len(bytes.TrimSpace(session.Schema)) > 0
		printer := newStreamPrinter(debug || !cmd.Stream || hasSchema, width)
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
		if hasSchema && reply.Result == providerschema.ResultStop {
			return printJSON(json.RawMessage(reply.Text()))
		} else if text := reply.Text(); text != "" && !printer.Streamed() {
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

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// WatchEvents watches the server's events, such as the changes to sessions
func (c chatClient) WatchEvents(ctx context.Context, fn func(eventschema.Event) error) error {
	return c.events.WatchEvents(ctx, fn)
}

// ToolNames returns the canonical names of the server's tools, by the names
// models call them by
func (c chatClient) ToolNames(ctx context.Context) (map[string]string, error) {
	list, err := c.tools.ListTools(ctx, toolschema.ToolListRequest{})
	if err != nil {
		return nil, err
	}
	names := make(map[string]string, len(list.Body))
	for _, tool := range list.Body {
		if namespace, name, ok := strings.Cut(tool.Canonical, toolschema.ToolSeparator); ok {
			names[toolschema.ModelToolName(namespace, name)] = tool.Canonical
		}
	}
	return names, nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// newToolClient returns a client of the server's toolkit, from the
// command's endpoint and options
func newToolClient(ctx server.Cmd) (*toolclient.Client, error) {
	endpoint, opts, err := ctx.ClientEndpoint()
	if err != nil {
		return nil, err
	}
	return toolclient.New(endpoint, opts...)
}

// newEventClient returns a client of the server's events, from the
// command's endpoint and options
func newEventClient(ctx server.Cmd) (*eventclient.Client, error) {
	endpoint, opts, err := ctx.ClientEndpoint()
	if err != nil {
		return nil, err
	}
	return eventclient.New(endpoint, opts...)
}
