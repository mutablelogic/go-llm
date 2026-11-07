//go:build !client

package main

import (
	"context"
	"encoding/json"
	"errors"
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
	registryschema "github.com/mutablelogic/go-llm/registry/schema"
	toolkit "github.com/mutablelogic/go-llm/toolkit"
	server "github.com/mutablelogic/go-server"
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
func newServerMessaging(ctx server.Cmd, tools *toolkit.Toolkit, sessions *atomic.Pointer[registry.Registry], opts ...messaging.Opt) (*messaging.Messaging, error) {
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
		msg, err := providerschema.NewMessage("user", message.Text)
		if err != nil {
			return err
		}
		_, err = r.Chat(ctx, channel.Session, []llm.Message{msg}, nil)
		return err
	}
	opts = append([]messaging.Opt{messaging.WithTracer(ctx.Tracer()), messaging.WithProvider(bot), messaging.WithChat(chat)}, opts...)
	return messaging.New(&channelStore{file: file}, opts...)
}

// deliverEvent sends the event's outcome to the channels of the session
// it's for, when it's to be told to the person, rather than ignored, or
// failed, and there's messaging, and returns whether it was sent to any
func deliverEvent(ctx context.Context, m *messaging.Messaging, session uuid.UUID, event *registryschema.Event) (bool, error) {
	if m == nil || event == nil || event.Status != registryschema.EventPending || strings.TrimSpace(event.Message) == "" {
		return false, nil
	}
	sent, err := m.Send(ctx, session, event.Message)
	if errors.Is(err, llm.ErrNotFound) {
		return false, nil
	}
	return len(sent) > 0, err
}
