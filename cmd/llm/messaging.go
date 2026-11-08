//go:build !client

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/url"
	"path/filepath"
	"strings"
	"sync/atomic"
	"uuid"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	messaging "github.com/mutablelogic/go-llm/messaging"
	schema "github.com/mutablelogic/go-llm/messaging/schema"
	telegram "github.com/mutablelogic/go-llm/messaging/telegram"
	providerschema "github.com/mutablelogic/go-llm/provider/schema"
	registry "github.com/mutablelogic/go-llm/registry"
	resource "github.com/mutablelogic/go-llm/resource"
	resourceschema "github.com/mutablelogic/go-llm/resource/schema"
	toolkit "github.com/mutablelogic/go-llm/toolkit"
	server "github.com/mutablelogic/go-server"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// channelStore is a store of the channels in a file of JSON, under one key
type channelStore struct {
	file *jsonFile
}

var _ messaging.Store = (*channelStore)(nil)

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// TelegramCredential is the key of the credential holding the Telegram
	// bot's token, from BotFather, which adds the Telegram provider
	TelegramCredential = "TELEGRAM_BOT_TOKEN"

	// channelsFile is the name of the file which the server keeps the
	// channels in, in the directory of the command's defaults, and
	// channelsKey is the key they're under
	channelsFile = "channels.json"
	channelsKey  = "messaging.channels"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

func (s *channelStore) Load(context.Context) ([]schema.Channel, error) {
	value, err := s.file.Lookup(channelsKey)
	if err != nil || value == nil {
		return nil, err
	}
	data, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	var channels []schema.Channel
	if err := json.Unmarshal(data, &channels); err != nil {
		return nil, llm.ErrInternalServerError.Withf("%s: %v", channelsFile, err)
	}
	return channels, nil
}

func (s *channelStore) Save(_ context.Context, channels []schema.Channel) error {
	if len(channels) == 0 {
		return s.file.Set(channelsKey, nil)
	}
	return s.file.Set(channelsKey, channels)
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// newServerMessaging returns the messaging, with the providers whose
// credentials are set, such as Telegram's bot token, whose channels are
// kept in a file, beside the providers, and whose messages are chatted in
// their channel's session, by the registry, once it's made, or nil, when no
// provider's credentials are set, or the providers can't be reached, which
// is logged
func newServerMessaging(ctx server.Cmd, tools *toolkit.Toolkit, sessions *atomic.Pointer[registry.Registry], res *resource.Manager, opts ...messaging.Opt) (*messaging.Messaging, error) {
	token, err := tools.Credential(ctx.Context(), TelegramCredential)
	if err != nil {
		ctx.Logger().WarnContext(ctx.Context(), "messaging not added, as its credential can't be read", "provider", telegram.Name, "credential", TelegramCredential, "error", err.Error())
		return nil, nil
	}
	bot, err := telegram.New(ctx.Context(), token, tools.ClientOpts()...)
	if err != nil {
		ctx.Logger().ErrorContext(ctx.Context(), "messaging not added, as the bot can't be reached", "provider", telegram.Name, "error", err.Error())
		return nil, nil
	}
	ctx.Logger().InfoContext(ctx.Context(), "messaging provider added", "provider", telegram.Name, "bot", bot.Username())

	dir, err := serverDir(ctx)
	if err != nil {
		return nil, err
	}
	file, err := newJSONFile(filepath.Join(dir, channelsFile))
	if err != nil {
		return nil, err
	}

	// A message from a channel is chatted in its session, whose reply the
	// registry sends to the session's channels
	chat := func(ctx context.Context, channel schema.Channel, message messaging.Message) error {
		r := sessions.Load()
		if r == nil {
			return llm.ErrServiceUnavailable.With("registry isn't running")
		}
		msg, err := chatMessage(ctx, res, channel.Session, message)
		if err != nil {
			return err
		}
		_, err = r.Chat(ctx, channel.Session, []llm.Message{msg}, nil)
		return err
	}
	opts = append([]messaging.Opt{messaging.WithTracer(ctx.Tracer()), messaging.WithProvider(bot), messaging.WithChat(chat)}, opts...)
	return messaging.New(&channelStore{file: file}, opts...)
}

// openFile returns the function which opens a session's file, which is one
// of the resources, by its URI, to send it to the session's channels
func openFile(res *resource.Manager) messaging.OpenFunc {
	return func(ctx context.Context, session uuid.UUID, uri string) (messaging.File, io.ReadCloser, error) {
		hash, err := resourceschema.ParseURI(uri)
		if err != nil {
			return messaging.File{}, nil, err
		}
		r, content, err := res.Open(ctx, resourceschema.ResourceKey{Hash: hash, Session: &session})
		if err != nil {
			return messaging.File{}, nil, err
		}
		return messaging.File{URI: r.URI(), Name: r.Name, Type: r.Type, Size: r.Size}, content, nil
	}
}

// chatMessage returns the message from a channel's chat, as the person's
// message in its session: the files sent with it, such as photos, or voice
// messages, which are kept as the session's files, as they're downloaded,
// and attached to it, as the web UI attaches them, and its text
func chatMessage(ctx context.Context, res *resource.Manager, session uuid.UUID, message messaging.Message) (*providerschema.Message, error) {
	content := make([]providerschema.ContentBlock, 0, len(message.Attachments)+1)
	var unavailable, names []string
	for _, attachment := range message.Attachments {
		if attachment.Unavailable != "" || attachment.Open == nil {
			unavailable = append(unavailable, fmt.Sprintf("%s wasn't received, as %s", attachment.Name, attachment.Unavailable))
			continue
		} else if res == nil {
			break
		}
		kept, err := keepAttachment(ctx, res, session, attachment)
		if err != nil {
			return nil, err
		}
		u, err := url.Parse(kept.URI())
		if err != nil {
			return nil, err
		}
		content = append(content, providerschema.ContentBlock{Resource: &providerschema.Resource{ResourceMeta: providerschema.ResourceMeta{ContentType: kept.Type, URL: u}}})
		names = append(names, fmt.Sprintf("%s (%s)", kept.Name, kept.URI()))
	}
	text := strings.TrimSpace(message.Text)
	if text != "" {
		content = append(content, providerschema.ContentBlock{Text: types.Ptr(text)})
	}

	// The text which is the caption of the files, which was sent with them,
	// rather than as a message of its own, which the model is told
	var notes []string
	if text != "" && message.Caption && len(names) > 0 {
		notes = append(notes, "The person's text is the caption of "+list(names)+", which they sent it with.")
	}

	// The files which weren't received, such as as they're too large, which
	// the model is told, so it can tell the person
	if notes = append(notes, unavailable...); len(notes) > 0 {
		content = append(content, providerschema.ContentBlock{Text: types.Ptr("<system-reminder>\n" + strings.Join(notes, "\n") + "\n</system-reminder>")})
	}
	if len(content) == 0 {
		return nil, llm.ErrBadParameter.With("the message has no text, and its files couldn't be kept")
	}
	return &providerschema.Message{MessageMeta: providerschema.MessageMeta{Sender: "user", Content: content}}, nil
}

// keepAttachment downloads the file, and keeps it, as it's read, as a file
// of the session, with its name, its type, or the one detected from its
// content, and name, when it has none, and its description
func keepAttachment(ctx context.Context, res *resource.Manager, session uuid.UUID, attachment messaging.Attachment) (resourceschema.Resource, error) {
	content, err := attachment.Open(ctx)
	if err != nil {
		return resourceschema.Resource{}, err
	}
	defer content.Close()
	kept, _, err := res.Create(ctx, resourceschema.ResourceMeta{Name: attachment.Name, Type: attachment.Type, Description: attachment.Description, Session: &session}, content)
	return kept, err
}

// list returns the things, as a phrase, such as "a", "a and b", or "a, b
// and c"
func list(things []string) string {
	if len(things) == 1 {
		return things[0]
	}
	return strings.Join(things[:len(things)-1], ", ") + " and " + things[len(things)-1]
}

// replyFiles returns the URIs of the files a reply made which are sent with
// it to its session's channels, which are its media: pictures, audio, such
// as speech, and video. Files which a model reads with, such as transcripts,
// and text an image was read as, aren't, as they're how it worked out its
// reply, rather than part of it, and the model sends one, when it means to,
// with messaging's send tool.
func replyFiles(reply registry.Reply) []string {
	var files []string
	for _, resource := range reply.Resources {
		if isMedia(resource.Type) {
			files = append(files, resource.URI)
		}
	}
	return files
}

// isMedia returns whether the type is of media: an image, audio, or video
func isMedia(contentType string) bool {
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return false
	}
	kind, _, _ := strings.Cut(strings.ToLower(mediaType), "/")
	switch kind {
	case "image", "audio", "video":
		return true
	}
	return false
}
