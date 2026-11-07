package cmd

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"uuid"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	eventclient "github.com/mutablelogic/go-llm/broadcaster/httpclient"
	eventschema "github.com/mutablelogic/go-llm/broadcaster/schema"
	memoryclient "github.com/mutablelogic/go-llm/memory/httpclient"
	memoryschema "github.com/mutablelogic/go-llm/memory/schema"
	providerschema "github.com/mutablelogic/go-llm/provider/schema"
	chattui "github.com/mutablelogic/go-llm/registry/cmd/tui"
	httpclient "github.com/mutablelogic/go-llm/registry/httpclient"
	schema "github.com/mutablelogic/go-llm/registry/schema"
	resourceclient "github.com/mutablelogic/go-llm/resource/httpclient"
	resourceschema "github.com/mutablelogic/go-llm/resource/schema"
	server "github.com/mutablelogic/go-server"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// ChatCmd asks a session's model a question, in the session, which keeps the
// question, and the reply, or, without a question, runs an interface in the
// terminal to chat in the session
type ChatCmd struct {
	Text    []string  `arg:"" name:"text" help:"The question to ask, which can be more than one argument, or, without one, chat in the terminal." optional:""`
	Attach  []string  `name:"attach" short:"a" aliases:"file" sep:"none" help:"A file the question is about, or a resource, by its URI, resource:///<hash>, which can be given more than once. A file of text, such as code or subtitles, is sent as text, with its name, and anything else, such as an image or a PDF, is attached as a resource of the session, which is uploaded, unless it was before, and which the model must accept." optional:""`
	Session uuid.UUID `name:"session" help:"Session identifier, which is a UUID, or the default session, which this is then." optional:""`
	Stream  bool      `name:"stream" negatable:"" default:"true" help:"Print the reply as it's generated, or with --no-stream, once it's complete, rendered as markdown on a terminal."`
}

// chatClient is the client of the registry's API, of its events, and of
// the memory's API, which the interface chats with
type chatClient struct {
	*httpclient.Client
	events *eventclient.Client
	memory *memoryclient.Client
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
	if text == "" && len(cmd.Attach) == 0 && width > 0 {
		if err := withClient(ctx, "llm.ChatCmd", func(reqctx context.Context, client *httpclient.Client) error {
			events, err := newEventClient(ctx)
			if err != nil {
				return err
			}
			memory, err := newMemoryClient(ctx)
			if err != nil {
				return err
			}
			return chattui.Chat(reqctx, chatClient{client, events, memory}, id, debug)
		}); err != nil {
			return err
		}
		return ctx.Set(defaultSessionKey, id.String())
	} else if text == "" {
		return fmt.Errorf("missing question")
	}
	resources, err := newResourceClient(ctx)
	if err != nil {
		return err
	}
	return withClient(ctx, "llm.ChatCmd", func(reqctx context.Context, client *httpclient.Client) error {
		// The question, whose files which aren't text are uploaded as
		// resources of the session, and attached by their URIs, so the
		// session keeps them once, rather than with their data
		message, err := askMessage(text, cmd.Attach, func(file string) (string, error) {
			return uploadResource(reqctx, resources, id, file)
		})
		if err != nil {
			return err
		}

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

// GetDetails returns the memory's details, such as the names of the person
// and the assistant
func (c chatClient) GetDetails(ctx context.Context) (*memoryschema.Details, error) {
	return c.memory.GetDetails(ctx)
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// newMemoryClient returns a client of the memory's API, from the command's
// endpoint and options
func newMemoryClient(ctx server.Cmd) (*memoryclient.Client, error) {
	endpoint, opts, err := ctx.ClientEndpoint()
	if err != nil {
		return nil, err
	}
	return memoryclient.New(endpoint, opts...)
}

// newEventClient returns a client of the server's events, from the
// command's endpoint and options
// newResourceClient returns a client of the resources' API, from the
// command's endpoint and options
func newResourceClient(ctx server.Cmd) (*resourceclient.Client, error) {
	endpoint, opts, err := ctx.ClientEndpoint()
	if err != nil {
		return nil, err
	}
	return resourceclient.New(endpoint, opts...)
}

// uploadResource returns the URI of the resource whose content is the
// file's, in the session, or on its own, which a chat links to its session,
// or, without a session, on its own, or otherwise uploads the file as one,
// in the session, or on its own, so a file which was uploaded before isn't
// uploaded again. The file is hashed, to find the resource, without sending
// it.
func uploadResource(ctx context.Context, client *resourceclient.Client, session uuid.UUID, file string) (string, error) {
	f, err := os.Open(file)
	if err != nil {
		return "", err
	}
	defer f.Close()

	// The resource with the file's content, in the session, or on its own
	hash := sha256.New()
	if _, err := io.Copy(hash, f); err != nil {
		return "", err
	}
	sum := hex.EncodeToString(hash.Sum(nil))
	scopes := []resourceschema.ResourceRequest{{}}
	if session != uuid.Nil() {
		scopes = []resourceschema.ResourceRequest{{Session: session.String()}, {}}
	}
	for _, scope := range scopes {
		resource, err := client.GetResource(ctx, sum, scope)
		if err == nil {
			return resource.URI(), nil
		} else if status, ok := llm.HTTPStatus(err); !ok || status != http.StatusNotFound {
			return "", fmt.Errorf("%s: %w", file, err)
		}
	}

	// Otherwise it's uploaded, from the start
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return "", err
	}
	req := resourceschema.ResourceCreateRequest{}
	if session != uuid.Nil() {
		req.Session = session.String()
	}
	resource, err := client.CreateResource(ctx, req, filepath.Base(file), f)
	if err != nil {
		return "", fmt.Errorf("%s: %w", file, err)
	}
	return resource.URI(), nil
}

// newEventClient returns a client of the events' API
func newEventClient(ctx server.Cmd) (*eventclient.Client, error) {
	endpoint, opts, err := ctx.ClientEndpoint()
	if err != nil {
		return nil, err
	}
	return eventclient.New(endpoint, opts...)
}
