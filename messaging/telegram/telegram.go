// Package telegram is the Telegram messaging provider, a bot, whose chats
// are channels. It receives messages by long polling, so the server needs
// no address on the internet.
package telegram

import (
	"context"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	// Packages
	client "github.com/mutablelogic/go-client"
	messaging "github.com/mutablelogic/go-llm/messaging"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Telegram is a Telegram bot, whose chats are channels
type Telegram struct {
	client   *client.Client
	username string

	// The identifier of the next update, which only Receive changes
	offset int64
}

var _ messaging.Provider = (*Telegram)(nil)
var _ messaging.Typer = (*Telegram)(nil)

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// Name is the provider's name
	Name = "telegram"

	// pollTimeout is how long, in seconds, a request for updates waits for
	// one, before it returns none
	pollTimeout = 30

	// maxText is the most characters of a message, beyond which a text is
	// sent as more than one, which is less than Telegram's 4,096, as it
	// counts some characters, such as emoji, as two
	maxText = 4000
)

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// New returns the bot with the token, from BotFather, with the client
// options, such as the user agent, or an endpoint for tests, once it's
// checked the token, and has the bot's username, which its links have
func New(ctx context.Context, token string, opts ...client.ClientOpt) (*Telegram, error) {
	c, err := newClient(token, opts...)
	if err != nil {
		return nil, err
	}
	me, err := call[user](ctx, c, "getMe", struct{}{})
	if err != nil {
		return nil, err
	}
	return &Telegram{client: c, username: me.Username}, nil
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Name returns the provider's name, telegram
func (*Telegram) Name() string {
	return Name
}

// Username returns the bot's username
func (t *Telegram) Username() string {
	return t.username
}

// Link returns the link which opens a chat with the bot, whose Start
// button sends it the code
func (t *Telegram) Link(code string) string {
	return "https://t.me/" + url.PathEscape(t.username) + "?start=" + url.QueryEscape(code)
}

// Receive calls the function with each message people send the bot, until
// the context is cancelled, when it returns nil, or updates can't be read.
// Messages from bots, and without text, are skipped. A message is received
// once, whatever the function returns.
func (t *Telegram) Receive(ctx context.Context, fn func(context.Context, messaging.Message) error) error {
	for {
		pollCtx, cancel := context.WithTimeout(ctx, (pollTimeout+15)*time.Second)
		updates, err := call[[]update](pollCtx, t.client, "getUpdates", getUpdatesRequest{
			Offset:         t.offset,
			Timeout:        pollTimeout,
			AllowedUpdates: []string{"message"},
		}, client.OptNoTimeout())
		cancel()
		if ctx.Err() != nil {
			return nil
		} else if err != nil {
			return err
		}
		for _, u := range updates {
			t.offset = max(t.offset, u.ID+1)
			if m, ok := toMessage(u.Message); ok {
				_ = fn(ctx, m)
			}
		}
	}
}

// Send sends the text to the chat, as more than one message, when it's
// longer than one can be, broken between paragraphs, lines, or words
func (t *Telegram) Send(ctx context.Context, address, text string) error {
	for _, part := range split(text, maxText) {
		if _, err := call[message](ctx, t.client, "sendMessage", sendMessageRequest{ChatID: address, Text: part}); err != nil {
			return err
		}
	}
	return nil
}

// Typing shows "typing…" in the chat, for five seconds, or until a message
// is sent
func (t *Telegram) Typing(ctx context.Context, address string) error {
	_, err := call[bool](ctx, t.client, "sendChatAction", sendChatActionRequest{ChatID: address, Action: "typing"})
	return err
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// toMessage returns the message, as messaging has it, and whether it's one
// a person sent, with text
func toMessage(m *message) (messaging.Message, bool) {
	if m == nil || m.From == nil || m.From.IsBot || strings.TrimSpace(m.Text) == "" {
		return messaging.Message{}, false
	}
	result := messaging.Message{
		Address: strconv.FormatInt(m.Chat.ID, 10),
		Title:   chatTitle(m.Chat),
		From:    strings.TrimSpace(m.From.FirstName + " " + m.From.LastName),
		Text:    m.Text,
		Time:    time.Unix(m.Date, 0).UTC(),
	}
	result.Code = startCode(m.Text)
	return result, true
}

// chatTitle returns the chat's name: a group's title, or the person's
func chatTitle(c chat) string {
	if c.Title != "" {
		return c.Title
	}
	if name := strings.TrimSpace(c.FirstName + " " + c.LastName); name != "" {
		return name
	}
	return c.Username
}

// startCode returns the code of a /start command, such as /start code, or
// /start@bot code, in a group, which the bot's link sends, or none
func startCode(text string) string {
	fields := strings.Fields(text)
	if len(fields) != 2 {
		return ""
	}
	command, _, _ := strings.Cut(fields[0], "@")
	if command != "/start" {
		return ""
	}
	return fields[1]
}

// split returns the text in parts of at most n characters, broken after
// the last paragraph, line, or word in each, when there is one
func split(text string, n int) []string {
	var parts []string
	for utf8.RuneCountInString(text) > n {
		// The first n characters, and where to break them
		end := 0
		for range n {
			_, size := utf8.DecodeRuneInString(text[end:])
			end += size
		}
		head := text[:end]
		cut := end
		for _, sep := range []string{"\n\n", "\n", " "} {
			if i := strings.LastIndex(head, sep); i > 0 {
				cut = i + len(sep)
				break
			}
		}
		if part := strings.TrimSpace(text[:cut]); part != "" {
			parts = append(parts, part)
		}
		text = text[cut:]
	}
	if part := strings.TrimSpace(text); part != "" {
		parts = append(parts, part)
	}
	return parts
}
