// Package telegram implements [ui.ChatUI] for Telegram bots using telebot v4.
package telegram

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
	"time"

	// Packages
	"github.com/mutablelogic/go-llm/pkg/ui"
	tele "gopkg.in/telebot.v4"
)

///////////////////////////////////////////////////////////////////////////////
// CONSTANTS

const (
	// Minimum interval between streaming edits to respect Telegram rate limits.
	editInterval = time.Second

	// Placeholder text shown while waiting for the first streaming chunk.
	streamPlaceholder = "..."
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Telegram implements [ui.ChatUI] for the Telegram Bot API.
type Telegram struct {
	bot    *tele.Bot
	events chan ui.Event
	done   chan struct{}
}

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// New creates a Telegram bot UI with the given token. It starts long-polling
// in a background goroutine and returns immediately.
func New(token string) (*Telegram, error) {
	bot, err := tele.NewBot(tele.Settings{
		Token:  token,
		Poller: &tele.LongPoller{Timeout: 10 * time.Second},
	})
	if err != nil {
		return nil, fmt.Errorf("telegram: %w", err)
	}

	t := &Telegram{
		bot:    bot,
		events: make(chan ui.Event, 32),
		done:   make(chan struct{}),
	}

	// Register handlers.
	bot.Handle(tele.OnText, t.onText)
	bot.Handle(tele.OnDocument, t.onDocument)
	bot.Handle(tele.OnAudio, t.onAudio)
	bot.Handle(tele.OnVoice, t.onVoice)
	bot.Handle(tele.OnPhoto, t.onPhoto)
	bot.Handle(tele.OnVideo, t.onVideo)
	bot.Handle(tele.OnVideoNote, t.onVideoNote)

	// Start polling in the background.
	go func() {
		bot.Start()
		close(t.done)
	}()

	return t, nil
}

///////////////////////////////////////////////////////////////////////////////
// ChatUI IMPLEMENTATION

// Receive blocks until the next incoming event, context cancellation, or
// shutdown. It returns io.EOF when the bot is stopped.
func (t *Telegram) Receive(ctx context.Context) (ui.Event, error) {
	select {
	case evt := <-t.events:
		return evt, nil
	case <-ctx.Done():
		return ui.Event{}, ctx.Err()
	case <-t.done:
		return ui.Event{}, io.EOF
	}
}

// Close stops the bot poller and waits for it to finish.
func (t *Telegram) Close() error {
	t.bot.Stop()
	<-t.done
	return nil
}

///////////////////////////////////////////////////////////////////////////////
// TELEBOT HANDLERS

func (t *Telegram) onText(c tele.Context) error {
	evt := t.textEvent(c)
	select {
	case t.events <- evt:
	default:
		// Drop if the consumer isn't keeping up.
	}
	return nil
}

func (t *Telegram) onDocument(c tele.Context) error {
	msg := c.Message()
	if msg == nil || msg.Document == nil {
		return nil
	}
	doc := msg.Document
	return t.emitAttachment(c, &doc.File, doc.FileName, doc.MIME, msg.Caption)
}

func (t *Telegram) onAudio(c tele.Context) error {
	msg := c.Message()
	if msg == nil || msg.Audio == nil {
		return nil
	}
	a := msg.Audio
	filename := a.FileName
	if filename == "" {
		filename = "audio" + mimeToExt(a.MIME, ".mp3")
	}
	return t.emitAttachment(c, &a.File, filename, a.MIME, msg.Caption)
}

func (t *Telegram) onVoice(c tele.Context) error {
	msg := c.Message()
	if msg == nil || msg.Voice == nil {
		return nil
	}
	v := msg.Voice
	mime := v.MIME
	if mime == "" {
		mime = "audio/ogg"
	}
	return t.emitAttachment(c, &v.File, "voice"+mimeToExt(mime, ".ogg"), mime, msg.Caption)
}

func (t *Telegram) onPhoto(c tele.Context) error {
	msg := c.Message()
	if msg == nil || msg.Photo == nil {
		return nil
	}
	return t.emitAttachment(c, &msg.Photo.File, "photo.jpg", "image/jpeg", msg.Caption)
}

func (t *Telegram) onVideo(c tele.Context) error {
	msg := c.Message()
	if msg == nil || msg.Video == nil {
		return nil
	}
	v := msg.Video
	filename := v.FileName
	if filename == "" {
		filename = "video" + mimeToExt(v.MIME, ".mp4")
	}
	mime := v.MIME
	if mime == "" {
		mime = "video/mp4"
	}
	return t.emitAttachment(c, &v.File, filename, mime, msg.Caption)
}

func (t *Telegram) onVideoNote(c tele.Context) error {
	msg := c.Message()
	if msg == nil || msg.VideoNote == nil {
		return nil
	}
	return t.emitAttachment(c, &msg.VideoNote.File, "videonote.mp4", "video/mp4", msg.Caption)
}

// emitAttachment downloads a file into memory and pushes a ui.EventAttachment.
// The download is buffered so the data survives past the telebot handler return
// (the HTTP response body is closed after the handler returns).
func (t *Telegram) emitAttachment(c tele.Context, file *tele.File, filename, mime, caption string) error {
	ctx := newContext(c.Bot(), c.Chat(), c.Sender())

	rc, err := c.Bot().File(file)
	if err != nil {
		c.Send(fmt.Sprintf("Error downloading file: %v", err))
		return fmt.Errorf("telegram: downloading file: %w", err)
	}
	data, err := io.ReadAll(rc)
	rc.Close()
	if err != nil {
		c.Send(fmt.Sprintf("Error reading file: %v", err))
		return fmt.Errorf("telegram: reading file: %w", err)
	}

	evt := ui.Event{
		Type:    ui.EventAttachment,
		Context: ctx,
		Text:    caption,
		Attachments: []ui.InAttachment{{
			Filename: filename,
			Type:     mime,
			Data:     bytes.NewReader(data),
		}},
	}

	select {
	case t.events <- evt:
	default:
		c.Send("Error: event queue full, attachment dropped")
	}
	return nil
}

// mimeToExt returns a file extension for common MIME types, or the fallback.
func mimeToExt(mime, fallback string) string {
	switch mime {
	case "audio/mpeg", "audio/mp3":
		return ".mp3"
	case "audio/ogg":
		return ".ogg"
	case "audio/wav", "audio/x-wav":
		return ".wav"
	case "audio/flac":
		return ".flac"
	case "audio/aac":
		return ".aac"
	case "audio/mp4", "audio/m4a":
		return ".m4a"
	case "video/mp4":
		return ".mp4"
	case "video/webm":
		return ".webm"
	case "image/jpeg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/gif":
		return ".gif"
	case "image/webp":
		return ".webp"
	default:
		return fallback
	}
}

// textEvent converts a telebot text message into a ui.Event, parsing
// slash commands (e.g. "/model gemini-2.5-flash") into EventCommand.
func (t *Telegram) textEvent(c tele.Context) ui.Event {
	ctx := newContext(c.Bot(), c.Chat(), c.Sender())
	text := c.Text()

	evt := ui.Event{
		Context: ctx,
		Text:    text,
	}

	if strings.HasPrefix(text, "/") {
		parts := strings.Fields(text)
		evt.Type = ui.EventCommand
		evt.Command = strings.TrimPrefix(parts[0], "/")
		if len(parts) > 1 {
			evt.Args = parts[1:]
		}
	} else {
		evt.Type = ui.EventText
	}

	return evt
}

///////////////////////////////////////////////////////////////////////////////
// CONTEXT

// streamSegment captures a contiguous run of text from a single role.
// telegramContext implements [ui.Context] for a single Telegram conversation.
type telegramContext struct {
	api  tele.API
	chat *tele.Chat
	user *tele.User

	// Streaming state (guarded by mu).
	mu         sync.Mutex
	streamMsg  *tele.Message // current placeholder being edited
	streamRole string        // role of the current segment
	streamBuf  strings.Builder
	lastEdit   time.Time
}

func newContext(api tele.API, chat *tele.Chat, user *tele.User) *telegramContext {
	return &telegramContext{
		api:  api,
		chat: chat,
		user: user,
	}
}

// UserID returns the Telegram user ID as a string.
func (c *telegramContext) UserID() string {
	if c.user != nil {
		return strconv.FormatInt(c.user.ID, 10)
	}
	return ""
}

// UserName returns the user's display name (username, or first+last name).
func (c *telegramContext) UserName() string {
	if c.user == nil {
		return ""
	}
	if c.user.Username != "" {
		return c.user.Username
	}
	name := c.user.FirstName
	if c.user.LastName != "" {
		name += " " + c.user.LastName
	}
	return name
}

// ConversationID returns the Telegram chat ID as a string.
func (c *telegramContext) ConversationID() string {
	if c.chat != nil {
		return strconv.FormatInt(c.chat.ID, 10)
	}
	return ""
}

// SendText sends a plain-text message to the conversation.
func (c *telegramContext) SendText(_ context.Context, text string) error {
	_, err := c.api.Send(c.chat, text)
	return err
}

// SendMarkdown sends a Markdown-formatted message, converting it to
// Telegram entities via goldmark-telegram.
func (c *telegramContext) SendMarkdown(_ context.Context, markdown string) error {
	text, entities := markdownToEntities(markdown)
	if len(entities) > 0 {
		_, err := c.api.Send(c.chat, text, entities)
		return err
	}
	_, err := c.api.Send(c.chat, text)
	return err
}

// SendAttachment sends a document/file to the conversation.
func (c *telegramContext) SendAttachment(_ context.Context, att ui.OutAttachment) error {
	doc := &tele.Document{
		File:     tele.FromReader(att.Data),
		FileName: att.Filename,
		MIME:     att.Type,
	}
	_, err := c.api.Send(c.chat, doc)
	return err
}

// SetTyping sends (or ignores a stop for) the "typing" chat action.
func (c *telegramContext) SetTyping(_ context.Context, typing bool) error {
	if typing {
		return c.api.Notify(c.chat, tele.Typing)
	}
	return nil
}

// StreamStart begins a streaming message by sending a placeholder that
// will be edited in-place as chunks arrive.
func (c *telegramContext) StreamStart(_ context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.streamBuf.Reset()
	c.streamRole = ""

	msg, err := c.api.Send(c.chat, streamPlaceholder)
	if err != nil {
		return err
	}
	c.streamMsg = msg
	c.lastEdit = time.Now()
	return nil
}

// StreamChunk appends text to the streaming buffer and periodically
// edits the placeholder message with the accumulated content. When the
// role changes, the previous segment is finalised and sent as its own
// chat bubble, and a typing indicator is shown. The placeholder for the
// new segment is created lazily so the typing indicator remains visible.
func (c *telegramContext) StreamChunk(_ context.Context, role, text string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Role changed — finalise the previous segment.
	if c.streamRole != "" && role != c.streamRole {
		c.finaliseSegment()
		c.streamMsg = nil

		// Show typing indicator so the user knows more is coming.
		c.api.Notify(c.chat, tele.Typing) //nolint:errcheck
	}

	c.streamRole = role
	c.streamBuf.WriteString(text)

	// Create placeholder lazily on the first chunk of a new segment.
	if c.streamMsg == nil {
		if msg, err := c.api.Send(c.chat, streamPlaceholder); err == nil {
			c.streamMsg = msg
			c.lastEdit = time.Now()
		}
	}

	// Periodically edit the placeholder with a live preview.
	if c.streamMsg != nil && time.Since(c.lastEdit) >= editInterval {
		preview := c.streamBuf.String()
		if c.streamRole == "thinking" {
			preview = "💭 " + preview
		} else if c.streamRole == "tool" {
			preview = "🔧 " + preview
		}
		if preview != "" {
			if edited, err := c.api.Edit(c.streamMsg, preview); err == nil {
				c.streamMsg = edited
			}
			c.lastEdit = time.Now()
		}
	}
	return nil
}

// StreamEnd finalises the last streaming segment with full formatting.
func (c *telegramContext) StreamEnd(_ context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.streamMsg == nil && c.streamBuf.Len() == 0 {
		return nil
	}

	// If there's buffered content but no placeholder yet, create one.
	if c.streamMsg == nil && c.streamBuf.Len() > 0 {
		if msg, err := c.api.Send(c.chat, streamPlaceholder); err == nil {
			c.streamMsg = msg
		} else {
			return nil
		}
	}

	if c.streamBuf.Len() == 0 {
		c.api.Delete(c.streamMsg) //nolint:errcheck
		c.streamMsg = nil
		return nil
	}

	c.finaliseSegment()
	c.streamMsg = nil
	return nil
}

// finaliseSegment formats the current streamBuf according to its role
// and edits the placeholder message with the result. Must be called
// with mu held.
func (c *telegramContext) finaliseSegment() {
	content := strings.TrimSpace(c.streamBuf.String())
	c.streamBuf.Reset()

	if content == "" {
		c.api.Delete(c.streamMsg) //nolint:errcheck
		return
	}

	switch c.streamRole {
	case "thinking":
		c.editWithEntity(content, tele.EntityBlockquote)
	case "tool":
		c.editWithEntity("🔧 "+content, tele.EntityItalic)
	default:
		// Assistant: full markdown rendering.
		text, entities := markdownToEntities(content)
		if len(entities) > 0 {
			if edited, err := c.api.Edit(c.streamMsg, text, entities); err == nil {
				c.streamMsg = edited
			} else {
				c.api.Edit(c.streamMsg, text) //nolint:errcheck
			}
		} else {
			c.api.Edit(c.streamMsg, text) //nolint:errcheck
		}
	}
}

// editWithEntity edits the current placeholder with text wrapped in a
// single entity spanning the full message.
func (c *telegramContext) editWithEntity(text string, entityType tele.EntityType) {
	entities := tele.Entities{{
		Type:   entityType,
		Offset: 0,
		Length: utf16Len(text),
	}}
	if edited, err := c.api.Edit(c.streamMsg, text, entities); err == nil {
		c.streamMsg = edited
	} else {
		c.api.Edit(c.streamMsg, text) //nolint:errcheck
	}
}
