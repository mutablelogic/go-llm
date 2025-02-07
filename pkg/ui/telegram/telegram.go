package telegram

import (
	"context"
	"encoding/json"
	"errors"
	"log"

	// Packages
	telegram "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	llm "github.com/mutablelogic/go-llm"
)

/////////////////////////////////////////////////////////////////////
// TYPES

type Client struct {
	*telegram.BotAPI
	callback CallbackFunc
}

type Message interface {
	Sender() string
	Text() string
	Typing() error
	Reply(context.Context, string, bool) error
}

type message struct {
	client    *Client
	sender    string
	text      string
	chatid    int64
	messageid int
}

/////////////////////////////////////////////////////////////////////
// LIFECYCLE

func New(token string, opts ...Opt) (*Client, error) {
	opt, err := applyOpts(token, opts...)
	if err != nil {
		return nil, err
	}
	bot, err := telegram.NewBotAPI(opt.token)
	bot.Debug = opt.debug
	if err != nil {
		return nil, err
	}

	// Create a new telegram instance
	telegram := &Client{bot, opt.callback}

	// Return the instance
	return telegram, nil
}

/////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

func (t *Client) Name() string {
	return t.Self.UserName
}

func (t *Client) Run(ctx context.Context) error {
	config := telegram.NewUpdate(0)
	config.Timeout = 60
	updates := t.GetUpdatesChan(config)
FOR_LOOP:
	for {
		select {
		case <-ctx.Done():
			break FOR_LOOP
		case evt := <-updates:
			if evt.Message == nil {
				// NO-OP
			} else if err := t.handleMessage(ctx, evt.Message); err != nil {
				log.Printf("Error: %v\n", err)
			}
		}
	}

	// Return success
	return nil
}

/////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

func (t *Client) purgeIdleSessions() {
	// TODO
}

func (t *Client) handleMessage(ctx context.Context, update *telegram.Message) error {
	// Check for command
	if command := update.Command(); command != "" {
		return t.handleCommand(ctx, command, update.CommandArguments())
	}

	// Check callback
	if t.callback == nil {
		return llm.ErrInternalServerError.With("No callback")
	}

	// Make a new message
	message := &message{
		client:    t,
		sender:    update.From.UserName,
		text:      update.Text,
		chatid:    update.Chat.ID,
		messageid: update.MessageID,
	}

	// Callback
	if err := t.callback(ctx, message); err != nil {
		return errors.Join(err, message.Reply(ctx, err.Error(), false))
	}

	// Return success
	return nil
}

func (t *Client) handleCommand(ctx context.Context, cmd, args string) error {
	// TODO
	return nil
}

/////////////////////////////////////////////////////////////////////
// MESSAGE

func (message message) Sender() string {
	return message.sender
}

func (message message) Text() string {
	return message.text
}

func (message message) Typing() error {
	action := telegram.NewChatAction(message.chatid, telegram.ChatTyping)
	_, err := message.client.Send(action)
	return err
}

func (message message) Reply(ctx context.Context, text string, markdown bool) error {
	mode := telegram.ModeMarkdownV2
	text = telegram.EscapeText(mode, text)
	msg := telegram.NewMessage(message.chatid, text)
	msg.ReplyToMessageID = message.messageid
	msg.ParseMode = mode
	_, err := message.client.Send(msg)
	return err
}

func (message message) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"sender": message.sender,
		"text":   message.text,
	})
}

func (message message) String() string {
	data, err := json.MarshalIndent(message, "", "  ")
	if err != nil {
		return err.Error()
	}
	return string(data)
}
