package messaging

import (
	"context"
	"time"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Provider is a messaging service, such as Telegram, whose chats are
// channels: it receives the messages sent to it, and sends messages to a
// chat
type Provider interface {
	// Name returns the provider's name, such as telegram, which its
	// channels have
	Name() string

	// Link returns the link which the person opens to connect a chat, with
	// the code, which the provider's message from that chat then has, such
	// as https://t.me/bot?start=code
	Link(code string) string

	// Receive calls the function with each message sent to the provider,
	// one at a time, in the order they're sent, until the context is
	// cancelled, when it returns nil, or it can't go on. A message the
	// function returns an error for isn't received again.
	Receive(ctx context.Context, fn func(context.Context, Message) error) error

	// Send sends the text to the chat with the address, as more than one
	// message, when it's longer than one can be
	Send(ctx context.Context, address, text string) error
}

// Typer is a provider which can show that a reply is being written, such
// as Telegram's "typing…"
type Typer interface {
	// Typing shows that a reply is being written in the chat with the
	// address, for a few seconds, or until a message is sent
	Typing(ctx context.Context, address string) error
}

// Message is a message sent to a provider, from a chat
type Message struct {
	// Address is the chat it was sent in
	Address string

	// Title is the chat's name, such as the person's, or a group's
	Title string

	// From is who sent it, such as their first name
	From string

	// Code is the code it has, when it connects the chat, such as from a
	// link which Link returned, and none otherwise
	Code string

	// Text is what it says
	Text string

	// Time is when it was sent
	Time time.Time
}
