package messaging

import (
	"context"
	"io"
	"time"
	"uuid"
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
	// message, when it's longer than one can be, and returns the messages it
	// was sent as, in order, including those which were sent before an
	// error
	Send(ctx context.Context, address, text string) ([]Sent, error)
}

// Typer is a provider which can show that a reply is being written, such
// as Telegram's "typing…"
type Typer interface {
	// Typing shows that a reply is being written in the chat with the
	// address, for a few seconds, or until a message is sent
	Typing(ctx context.Context, address string) error
}

// FileSender is a provider which can send files, such as images, and
// speech
type FileSender interface {
	// SendFile sends the file, whose content the function opens, and is read
	// as it's sent, which the provider closes, and can open again, such as to
	// send it again, to the chat with the address, with the caption, which is
	// Markdown, when it has one, under it, or before it, when it's too long
	// to be its caption
	//
	// It returns the messages it was sent as, in order, such as the caption,
	// when it's a message of its own, and the file, including those which
	// were sent before an error
	SendFile(ctx context.Context, address string, file File, open func() (io.ReadCloser, error), caption string) ([]Sent, error)
}

// Sent is a message which was sent to a chat: its identifier, which the
// provider gave it, and what it was. That's text, a file, with the text as
// its caption, or without it, or a note in place of a file which couldn't
// be sent, such as one which is too large.
type Sent struct {
	ID      string `json:"id"`
	Text    bool   `json:"text,omitempty"`
	File    string `json:"file,omitempty"`
	Caption bool   `json:"caption,omitempty"`
	Note    string `json:"note,omitempty"`
}

// File is a session's file, which is sent to its channels
type File struct {
	// URI is the file's, such as resource:///<hash>
	URI string

	// Name is the file's name, such as speech.ogg
	Name string

	// Type is its MIME type, such as image/png
	Type string

	// Size is its size, in bytes
	Size int64
}

// OpenFunc opens the session's file with the URI, and returns what it is,
// and its content, which the caller closes. Returns ErrNotFound when the
// session has no file with it.
type OpenFunc func(ctx context.Context, session uuid.UUID, uri string) (File, io.ReadCloser, error)

// Outgoing is what's sent to a session's channels: text, which is
// Markdown, and the session's files, by their URIs, such as an image, or
// speech
type Outgoing struct {
	Text  string
	Files []string
}

// Attachment is a file sent with a message, such as a photo, a document,
// or a voice message: its name, its type, when it's known, or empty, so
// it's detected from its content, its size, when it's known, what it is,
// such as a photo which was compressed, and the function which opens it,
// which is downloaded as it's read, which the caller closes, or why it
// can't be, such as that it's too large, when it's unavailable
type Attachment struct {
	Name        string
	Type        string
	Size        int64
	Description string
	Open        func(context.Context) (io.ReadCloser, error)
	Unavailable string
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

	// Text is what it says, or the caption of its attachments
	Text string

	// Caption is whether the text is the caption of its attachments, which
	// was sent with them, rather than a message of its own
	Caption bool

	// Attachments are the files sent with it, such as photos, documents,
	// and voice messages, which are opened when they're kept
	Attachments []Attachment

	// Time is when it was sent
	Time time.Time
}
