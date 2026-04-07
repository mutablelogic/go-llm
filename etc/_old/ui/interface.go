// Package ui defines the interface for chat user interfaces.
//
// Implementations of [ChatUI] adapt different platforms (terminal, Telegram,
// Slack, WhatsApp, etc.) to a common event-driven chat model. The bot
// receives incoming events via [ChatUI.Receive] and sends responses
// (text, attachments, typing indicators) through a [Context] obtained
// from each event.
package ui

import (
	"context"
	"io"
	"net/url"
)

///////////////////////////////////////////////////////////////////////////////
// INTERFACES

// ChatUI is the top-level interface that every chat frontend must implement.
// It is an event source: callers loop over [Receive] to process incoming
// user activity.
type ChatUI interface {
	// Receive blocks until the next incoming event is available, the
	// context is cancelled, or the interface is closed. It returns
	// io.EOF when the interface is permanently closed (e.g. terminal
	// EOF, bot shutdown).
	Receive(ctx context.Context) (Event, error)

	// Close releases resources held by the interface (e.g. websocket
	// connections, terminal raw mode).
	Close() error
}

// Context represents the conversation context for a single event. It
// identifies the user, conversation, and provides methods for the bot
// to send responses back to the same conversation.
type Context interface {
	// UserID returns a platform-specific unique identifier for the user
	// who triggered the event (e.g. Telegram user ID, Slack member ID,
	// terminal username).
	UserID() string

	// UserName returns a human-readable display name for the user.
	UserName() string

	// ConversationID returns a unique identifier for the conversation
	// (e.g. Telegram chat ID, Slack channel ID, terminal session).
	ConversationID() string

	// SendText sends a text message to the conversation. The text may
	// contain Markdown formatting; the implementation should render it
	// appropriately for the platform (e.g. ANSI for terminals, native
	// Markdown for Telegram, mrkdwn for Slack).
	SendText(ctx context.Context, text string) error

	// SendMarkdown sends a Markdown-formatted message. Platforms that
	// support rich text should render it natively; others may fall back
	// to plain text or ANSI formatting.
	SendMarkdown(ctx context.Context, markdown string) error

	// SendAttachment sends a file or media attachment to the conversation.
	// The MIME type, filename, and reader must be provided.
	SendAttachment(ctx context.Context, att OutAttachment) error

	// SetTyping signals that the bot is "typing" (or processing). The
	// implementation should show a typing indicator appropriate to the
	// platform. Call with typing=true to start and typing=false to stop.
	// Implementations may ignore the stop call if the platform handles
	// it automatically.
	SetTyping(ctx context.Context, typing bool) error

	// StreamStart begins a new streaming message in the conversation.
	// Subsequent calls to StreamChunk append text to this message.
	// The typing indicator is automatically shown during streaming.
	StreamStart(ctx context.Context) error

	// StreamChunk appends a text chunk to the current streaming message.
	// Must be called after StreamStart. The text appears incrementally
	// in the conversation as it arrives. The role parameter identifies
	// the source of the chunk (e.g. "assistant", "thinking", "tool")
	// so the UI can style each segment appropriately.
	StreamChunk(ctx context.Context, role, text string) error

	// StreamEnd finalises the current streaming message. If the platform
	// supports Markdown, the complete text may be re-rendered with full
	// formatting at this point. The typing indicator is hidden.
	StreamEnd(ctx context.Context) error
}

///////////////////////////////////////////////////////////////////////////////
// EVENT TYPES

// EventType identifies the kind of incoming event.
type EventType int

const (
	EventText       EventType = iota // User sent a text message
	EventCommand                     // User sent a slash command (e.g. /model, /session)
	EventAttachment                  // User sent a file/media attachment
)

func (t EventType) String() string {
	switch t {
	case EventText:
		return "text"
	case EventCommand:
		return "command"
	case EventAttachment:
		return "attachment"
	default:
		return "unknown"
	}
}

// Event represents an incoming event from the user.
type Event struct {
	// Type identifies what kind of event this is.
	Type EventType

	// Context provides the conversation context and response methods.
	Context Context

	// Text contains the message text (for EventText) or the full
	// command string including arguments (for EventCommand,
	// e.g. "/model gemini-2.5-flash").
	Text string

	// Command contains the parsed command name without the leading
	// slash (for EventCommand only, e.g. "model").
	Command string

	// Args contains the parsed command arguments (for EventCommand
	// only, e.g. ["gemini-2.5-flash"]).
	Args []string

	// Attachments contains the files/media sent by the user
	// (for EventAttachment, may also accompany EventText).
	Attachments []InAttachment
}

///////////////////////////////////////////////////////////////////////////////
// ATTACHMENT TYPES

// InAttachment represents a file or media attachment received from the user.
type InAttachment struct {
	// Filename is the original filename, if available.
	Filename string

	// Type is the MIME type (e.g. "image/png", "application/pdf").
	Type string

	// URL is a reference URL for the attachment, if available.
	URL *url.URL

	// Data is the raw content of the attachment.
	Data io.Reader
}

// OutAttachment represents a file or media attachment to send to the user.
type OutAttachment struct {
	// Filename is the filename to present to the user.
	Filename string

	// Type is the MIME type (e.g. "image/png", "application/pdf").
	Type string

	// Data is the content to send.
	Data io.Reader
}
