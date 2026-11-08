package messaging

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"
	"uuid"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	eventschema "github.com/mutablelogic/go-llm/broadcaster/schema"
	schema "github.com/mutablelogic/go-llm/messaging/schema"
	toolkit "github.com/mutablelogic/go-llm/toolkit"
	jsonschema "github.com/mutablelogic/go-server/pkg/jsonschema"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// connectTool returns a link which connects a chat to the conversation
type connectTool struct {
	toolkit.Base
	messaging *Messaging
}

// listTool lists the conversation's channels
type listTool struct {
	toolkit.Base
	messaging *Messaging
}

// sendTool sends a message to the conversation's channels
type sendTool struct {
	toolkit.Base
	messaging *Messaging
}

// disconnectTool disconnects one of the conversation's channels
type disconnectTool struct {
	toolkit.Base
	messaging *Messaging
}

var _ llm.Tool = (*connectTool)(nil)
var _ llm.Tool = (*listTool)(nil)
var _ llm.Tool = (*sendTool)(nil)
var _ llm.Tool = (*disconnectTool)(nil)
var _ toolkit.Namespace = (*Messaging)(nil)

type connectInput struct {
	Provider string `json:"provider,omitempty" help:"The messaging service, such as telegram, or none, when there's only one" optional:""`
}

type connectOutput struct {
	Link    string `json:"link"`
	Expires string `json:"expires"`
	Note    string `json:"note"`
}

type listInput struct{}

type channelOutput struct {
	ID        string `json:"id"`
	Provider  string `json:"provider"`
	Chat      string `json:"chat,omitempty"`
	Connected string `json:"connected"`
}

type listOutput struct {
	Providers []string        `json:"providers"`
	Count     int             `json:"count"`
	Channels  []channelOutput `json:"channels"`
}

type sendInput struct {
	Text     string   `json:"text,omitempty" help:"What to send, as Markdown, which is the caption of the first file, when it's an image" optional:""`
	Files    []string `json:"files,omitempty" help:"The URIs of the conversation's files to send, such as resource:///<hash>, such as an image, or speech" optional:""`
	Channels []string `json:"channels,omitempty" help:"The identifiers of the channels to send it to, or none, to send it to all of the conversation's channels" optional:""`
}

// sendOutput is what was sent to each channel: its messages, in order, by
// their identifiers, and what each was, and why something wasn't sent
type sendOutput struct {
	Sent []channelSent `json:"sent"`
}

type channelSent struct {
	channelOutput
	Count    int    `json:"count"`
	Messages []Sent `json:"messages"`
	Error    string `json:"error,omitempty"`
}

type disconnectInput struct {
	ID string `json:"id" help:"The identifier of the channel to disconnect"`
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS - NAMESPACE

func (*Messaging) Name() string {
	return Name
}

// Description returns what messaging is
func (*Messaging) Description() string {
	return "Chats on messaging services, such as Telegram, connected to a conversation, so the person can talk to their assistant there"
}

// SystemPrompt returns how channels work, and when to use the tools
func (m *Messaging) SystemPrompt() string {
	return strings.ReplaceAll(messagingPrompt, "{providers}", strings.Join(m.Providers(), ", "))
}

// Notify does nothing, as the tools don't change
func (*Messaging) Notify(func(eventschema.EventAction, string)) {}

// Tools returns the messaging tools
func (m *Messaging) Tools() []llm.Tool {
	return []llm.Tool{
		&connectTool{messaging: m},
		&listTool{messaging: m},
		&sendTool{messaging: m},
		&disconnectTool{messaging: m},
	}
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS - TOOLS

func (*connectTool) Name() string { return "connect" }

func (*connectTool) Description() string {
	return "Returns a link which connects a chat, such as on Telegram, to this conversation, when the person opens it. Their messages there then come to this conversation, and your replies go there."
}

func (*connectTool) InputSchema() *jsonschema.Schema {
	return jsonschema.MustFor[connectInput]()
}

func (t *connectTool) Run(ctx context.Context, input json.RawMessage) (any, error) {
	var req connectInput
	if err := decode(input, &req); err != nil {
		return nil, err
	}
	session, err := conversation(ctx)
	if err != nil {
		return nil, err
	}
	link, expires, err := t.messaging.Connect(ctx, session, req.Provider)
	if err != nil {
		return nil, err
	}
	toolkit.Logf(ctx, "Making a link to connect a chat")
	return connectOutput{
		Link:    link,
		Expires: expires.UTC().Format(time.RFC3339),
		Note:    "Give the person the link, as it is, on a line of its own, which they open, or scan, and then press Start. It works once, within 15 minutes.",
	}, nil
}

func (*listTool) Name() string { return "list" }

func (*listTool) Description() string {
	return "Lists the chats, such as on Telegram, connected to this conversation, and the messaging services which can be connected."
}

func (*listTool) InputSchema() *jsonschema.Schema {
	return jsonschema.MustFor[listInput]()
}

func (t *listTool) Run(ctx context.Context, _ json.RawMessage) (any, error) {
	session, err := conversation(ctx)
	if err != nil {
		return nil, err
	}
	toolkit.Logf(ctx, "Looking at the connected chats")
	channels, err := t.messaging.List(ctx, session)
	if err != nil {
		return nil, err
	}
	return listOutput{Providers: t.messaging.Providers(), Count: len(channels), Channels: outputs(channels)}, nil
}

func (*sendTool) Name() string { return "send" }

func (*sendTool) Description() string {
	return "Sends a message, and the conversation's files, such as an image, or speech, to the chats connected to this conversation, such as on Telegram, which isn't your reply, as your replies, and the files you make with them, are sent there anyway. Returns, for each chat, the messages it was sent as, in order, each with its identifier, and whether it's text, a file, by its name, with the text as its caption, or a note in place of a file, and why anything wasn't sent."
}

func (*sendTool) InputSchema() *jsonschema.Schema {
	return jsonschema.MustFor[sendInput]()
}

func (t *sendTool) Run(ctx context.Context, input json.RawMessage) (any, error) {
	var req sendInput
	if err := decode(input, &req); err != nil {
		return nil, err
	}
	session, err := conversation(ctx)
	if err != nil {
		return nil, err
	}
	ids := make([]uuid.UUID, 0, len(req.Channels))
	for _, s := range req.Channels {
		id, err := uuid.Parse(strings.TrimSpace(s))
		if err != nil {
			return nil, llm.ErrBadParameter.Withf("invalid channel id %q", s)
		}
		ids = append(ids, id)
	}
	toolkit.Logf(ctx, "Sending a message")
	deliveries, err := t.messaging.Send(ctx, session, Outgoing{Text: req.Text, Files: req.Files}, ids...)
	if err != nil {
		return nil, err
	}

	// What was sent to each, and why something wasn't, which is an error,
	// when nothing was sent to any
	result := sendOutput{Sent: make([]channelSent, 0, len(deliveries))}
	var errs error
	sent := 0
	for _, delivery := range deliveries {
		output := channelSent{channelOutput: outputs([]schema.Channel{delivery.Channel})[0], Count: len(delivery.Messages), Messages: delivery.Messages}
		if output.Messages == nil {
			output.Messages = []Sent{}
		}
		if delivery.Err != nil {
			output.Error = delivery.Err.Error()
			errs = errors.Join(errs, delivery.Err)
		}
		sent += len(delivery.Messages)
		result.Sent = append(result.Sent, output)
	}
	if sent == 0 && errs != nil {
		return nil, errs
	}
	return result, nil
}

func (*disconnectTool) Name() string { return "disconnect" }

func (*disconnectTool) Description() string {
	return "Disconnects a chat from this conversation, by its channel's id, so messages there no longer come here, and your replies don't go there."
}

func (*disconnectTool) InputSchema() *jsonschema.Schema {
	return jsonschema.MustFor[disconnectInput]()
}

func (t *disconnectTool) Run(ctx context.Context, input json.RawMessage) (any, error) {
	var req disconnectInput
	if err := decode(input, &req); err != nil {
		return nil, err
	}
	session, err := conversation(ctx)
	if err != nil {
		return nil, err
	}
	id, err := uuid.Parse(strings.TrimSpace(req.ID))
	if err != nil {
		return nil, llm.ErrBadParameter.Withf("invalid channel id %q", req.ID)
	}

	// Only the conversation's channels can be disconnected
	channels, err := t.messaging.List(ctx, session)
	if err != nil {
		return nil, err
	}
	found := false
	for _, channel := range channels {
		found = found || channel.ID == id
	}
	if !found {
		return nil, llm.ErrNotFound.Withf("channel %q", id)
	}
	channel, err := t.messaging.Disconnect(ctx, id)
	if err != nil {
		return nil, err
	}
	toolkit.Logf(ctx, "Disconnecting a chat")
	return outputs([]schema.Channel{channel})[0], nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// conversation returns the session the tool runs in, or ErrBadParameter
// when it doesn't run in one
func conversation(ctx context.Context) (uuid.UUID, error) {
	session := toolkit.SessionFromContext(ctx).ID()
	if session == uuid.Nil() {
		return uuid.Nil(), llm.ErrBadParameter.With("chats are connected to a conversation, and this isn't one")
	}
	return session, nil
}

// decode decodes the tool's input, which can be empty
func decode(input json.RawMessage, v any) error {
	if len(input) == 0 {
		return nil
	} else if err := json.Unmarshal(input, v); err != nil {
		return llm.ErrBadParameter.With(err.Error())
	}
	return nil
}

// outputs returns the channels, as a model is told them
func outputs(channels []schema.Channel) []channelOutput {
	result := make([]channelOutput, 0, len(channels))
	for _, channel := range channels {
		result = append(result, channelOutput{
			ID:        channel.ID.String(),
			Provider:  channel.Provider,
			Chat:      channel.Title,
			Connected: channel.Created.UTC().Format(time.RFC3339),
		})
	}
	return result
}

// messagingPrompt is how the model uses channels, with the providers
const messagingPrompt = `# Messaging

Chats on messaging services ({providers}) can be connected to this conversation, as channels, so the person can talk to you there, such as from their phone.

- When the person asks to talk to you on one, connect it with ` + "`connect`" + `, and give them the link it returns, as it is, on a line of its own, which they open, or scan with their phone, as the web UI shows it as a QR code, and press Start.
- Every reply you write in this conversation, with the files you make with it, such as a picture, or speech, is sent to each of its channels, wherever the message came from, so don't use ` + "`send`" + ` to reply.
- Use ` + "`send`" + ` only for a message which isn't a reply, such as to one channel only, or to send one of the conversation's files, by its URI.
- While the conversation has channels, your replies are also read on a phone, so keep them short. Markdown is fine, as it's formatted for each channel.`
