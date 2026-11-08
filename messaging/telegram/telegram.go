// Package telegram is the Telegram messaging provider, a bot, whose chats
// are channels. It receives messages by long polling, so the server needs
// no address on the internet.
package telegram

import (
	"context"
	"fmt"
	"io"
	"mime"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	// Packages
	client "github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
	messaging "github.com/mutablelogic/go-llm/messaging"
	types "github.com/mutablelogic/go-server/pkg/types"
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
var _ messaging.FileSender = (*Telegram)(nil)

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// Name is the provider's name
	Name = "telegram"

	// pollTimeout is how long, in seconds, a request for updates waits for
	// one, before it returns none, and albumWait how long it waits for the
	// rest of an album, whose messages were split between requests
	pollTimeout = 30
	albumWait   = 1

	// maxCaption is the most characters of a file's caption, which is less
	// than Telegram's 1,024, as it counts some characters, such as emoji,
	// as two
	maxCaption = 1000

	// maxUpload is the largest file a bot can send, and maxPhoto the
	// largest image which is sent as a photo, rather than a document, and
	// maxDownload the largest file a bot can download
	maxUpload   = 50 << 20
	maxPhoto    = 10 << 20
	maxDownload = 20 << 20

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
	// An album, whose messages, which are updates of their own, can be
	// split between polls, is held, when it's the last of a poll, until the
	// next, which waits for its other messages briefly
	var held []*message
	deliver := func(group []*message) {
		if m, ok := t.toMessage(ctx, group); ok {
			_ = fn(ctx, m)
		}
	}
	for {
		timeout := pollTimeout
		if held != nil {
			timeout = albumWait
		}
		pollCtx, cancel := context.WithTimeout(ctx, time.Duration(timeout+15)*time.Second)
		updates, err := call[[]update](pollCtx, t.client, "getUpdates", getUpdatesRequest{
			Offset:         t.offset,
			Timeout:        timeout,
			AllowedUpdates: []string{"message"},
		}, client.OptNoTimeout())
		cancel()
		if ctx.Err() != nil {
			return nil
		} else if err != nil {
			if held != nil {
				deliver(held)
			}
			return err
		}

		// The messages of an album, which follow each other, are one
		// message, with all their photos, and videos
		groups := [][]*message{}
		if held != nil {
			groups, held = append(groups, held), nil
		}
		for _, u := range updates {
			t.offset = max(t.offset, u.ID+1)
			if u.Message == nil {
				continue
			} else if n := len(groups); n > 0 && sameAlbum(groups[n-1][0], u.Message) {
				groups[n-1] = append(groups[n-1], u.Message)
				continue
			}
			groups = append(groups, []*message{u.Message})
		}

		// The last, when it's an album, and there were updates, is held, for
		// more of its messages, and the others are delivered
		if n := len(groups); n > 0 && len(updates) > 0 && groups[n-1][0].MediaGroupID != "" {
			held, groups = groups[n-1], groups[:n-1]
		}
		for _, group := range groups {
			deliver(group)
		}
	}
}

// sameAlbum returns whether the message is in the album the other is, in
// the same chat
func sameAlbum(album, m *message) bool {
	return album.MediaGroupID != "" && album.MediaGroupID == m.MediaGroupID && album.Chat.ID == m.Chat.ID
}

// Send sends the text, which is Markdown, to the chat, formatted as
// Telegram's HTML, as more than one message, when it's longer than one can
// be, broken between paragraphs, or other blocks, and as plain text, when
// Telegram can't parse its HTML, and returns the messages it was sent as.
// Links don't have previews.
func (t *Telegram) Send(ctx context.Context, address, text string) ([]messaging.Sent, error) {
	var result []messaging.Sent
	for _, p := range parts(text, maxText) {
		req := sendMessageRequest{ChatID: address, Text: p.html, ParseMode: "HTML", LinkPreviewOptions: &linkPreviewOptions{IsDisabled: true}}
		sent, err := call[message](ctx, t.client, "sendMessage", req)
		if err != nil && strings.Contains(err.Error(), "can't parse entities") {
			req.Text, req.ParseMode = p.plain, ""
			sent, err = call[message](ctx, t.client, "sendMessage", req)
		}
		if err != nil {
			return result, err
		}
		result = append(result, messaging.Sent{ID: messageID(sent), Text: true})
	}
	return result, nil
}

// SendFile sends the file to the chat, as a photo, when it's an image
// Telegram shows as one, a voice message, when it's Ogg, or Opus, audio,
// a track, when it's MP3, or M4A, audio, and otherwise as a document, with
// the caption, which is Markdown, under it, formatted, when it fits, or as
// a message before it, when it doesn't, and returns the messages it was
// sent as. A caption whose formatting Telegram can't parse is sent as plain
// text, with the file again. A file which is larger than a bot can send is
// a note, with its name, after its caption.
func (t *Telegram) SendFile(ctx context.Context, address string, file messaging.File, open func() (io.ReadCloser, error), caption string) ([]messaging.Sent, error) {
	name := fileName(file)
	caption = strings.TrimSpace(caption)
	var result []messaging.Sent
	if file.Size > maxUpload {
		if caption != "" {
			sent, err := t.Send(ctx, address, caption)
			if result = append(result, sent...); err != nil {
				return result, err
			}
		}
		note, err := call[message](ctx, t.client, "sendMessage", sendMessageRequest{ChatID: address, Text: fmt.Sprintf("📎 %s is too large to send here (%s)", name, megabytes(file.Size))})
		if err != nil {
			return result, err
		}
		return append(result, messaging.Sent{ID: messageID(note), File: name, Note: "too large to send, so this is a note in its place"}), nil
	}

	// The caption, as Telegram's HTML, when it's one message, which fits, or
	// a message before it
	var html, plain string
	if caption != "" {
		if p := parts(caption, maxCaption); len(p) == 1 {
			html, plain = p[0].html, p[0].plain
		} else {
			sent, err := t.Send(ctx, address, caption)
			if result = append(result, sent...); err != nil {
				return result, err
			}
		}
	}
	sent, err := t.upload(ctx, address, file, name, open, html, "HTML")
	if err != nil && html != "" && strings.Contains(err.Error(), "can't parse entities") {
		sent, err = t.upload(ctx, address, file, name, open, plain, "")
	}
	if err != nil {
		return result, err
	}
	return append(result, messaging.Sent{ID: messageID(sent), File: name, Caption: html != ""}), nil
}

// upload opens the file, and sends it, with its caption, which is in the
// parse mode, when it has one, by the method for its kind, and returns the
// message it was sent as
func (t *Telegram) upload(ctx context.Context, address string, file messaging.File, name string, open func() (io.ReadCloser, error), caption, parseMode string) (message, error) {
	content, err := open()
	if err != nil {
		return message{}, err
	}
	defer content.Close()
	if caption == "" {
		parseMode = ""
	}
	f := types.File{Path: name, Body: io.NopCloser(content), ContentType: file.Type}
	switch kind(file) {
	case "photo":
		return upload[message](ctx, t.client, "sendPhoto", sendPhotoRequest{ChatID: address, Photo: f, Caption: caption, ParseMode: parseMode})
	case "voice":
		return upload[message](ctx, t.client, "sendVoice", sendVoiceRequest{ChatID: address, Voice: f, Caption: caption, ParseMode: parseMode})
	case "audio":
		return upload[message](ctx, t.client, "sendAudio", sendAudioRequest{ChatID: address, Audio: f, Caption: caption, ParseMode: parseMode})
	}
	return upload[message](ctx, t.client, "sendDocument", sendDocumentRequest{ChatID: address, Document: f, Caption: caption, ParseMode: parseMode})
}

// messageID returns the message's identifier, as text
func messageID(m message) string {
	return strconv.FormatInt(m.ID, 10)
}

// Typing shows "typing…" in the chat, for five seconds, or until a message
// is sent
func (t *Telegram) Typing(ctx context.Context, address string) error {
	_, err := call[bool](ctx, t.client, "sendChatAction", sendChatActionRequest{ChatID: address, Action: "typing"})
	return err
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// kind returns how the file is sent: as a photo, a voice message, audio,
// or a document
func kind(file messaging.File) string {
	mediaType, _, err := mime.ParseMediaType(file.Type)
	if err != nil {
		return "document"
	}
	switch mediaType {
	case "image/jpeg", "image/png", "image/webp":
		if file.Size <= maxPhoto {
			return "photo"
		}
	case "audio/ogg", "audio/opus":
		return "voice"
	case "audio/mpeg", "audio/mp3", "audio/mp4", "audio/m4a", "audio/x-m4a":
		return "audio"
	}
	return "document"
}

// fileName returns the file's name, or one of its type, such as file.png,
// when it has none
func fileName(file messaging.File) string {
	if name := strings.TrimSpace(file.Name); name != "" {
		return name
	}
	if extensions, _ := mime.ExtensionsByType(file.Type); len(extensions) > 0 {
		return "file" + extensions[0]
	}
	return "file"
}

// megabytes returns the size, in megabytes, such as 61.2 MB
func megabytes(size int64) string {
	return fmt.Sprintf("%.1f MB", float64(size)/(1<<20))
}

// toMessage returns the message, as messaging has it, and whether it's one
// a person sent, with text
func (t *Telegram) toMessage(ctx context.Context, group []*message) (messaging.Message, bool) {
	m := group[0]
	if m == nil || m.From == nil || m.From.IsBot {
		return messaging.Message{}, false
	}
	address := strconv.FormatInt(m.Chat.ID, 10)
	result := messaging.Message{
		Address: address,
		Title:   chatTitle(m.Chat),
		From:    strings.TrimSpace(m.From.FirstName + " " + m.From.LastName),
		Text:    m.Text,
		Time:    time.Unix(m.Date, 0).UTC(),
	}
	for _, other := range group {
		if result.Text == "" && strings.TrimSpace(other.Caption) != "" {
			result.Text, result.Caption = other.Caption, true
		}
	}
	result.Code = startCode(m.Text)

	// The files sent with it, and the others of its album, which are
	// downloaded once they're opened, so a chat which isn't connected has
	// nothing downloaded, apart from those which are too large for a bot to
	// download, which say so
	for _, other := range group {
		for _, a := range attachments(other) {
			attachment := messaging.Attachment{Name: a.Name, Type: a.Type, Size: a.Size, Description: a.Description}
			if result.Caption {
				// The caption of an album is of all its files
				attachment.Description = describe(result.Text, a.Description)
			}
			if a.Size > maxDownload {
				attachment.Unavailable = fmt.Sprintf("it's too large (%s), as a bot can only download files of up to 20 MB", megabytes(a.Size))
			} else {
				attachment.Open = t.opener(a.fileID)
			}
			result.Attachments = append(result.Attachments, attachment)
		}
	}
	if strings.TrimSpace(result.Text) == "" && len(result.Attachments) == 0 {
		return messaging.Message{}, false
	}
	return result, true
}

// attachment is a file sent with a message, by its file's identifier
type attachment struct {
	fileID      string
	Name        string
	Type        string
	Size        int64
	Description string
}

// describe returns the description of a file: its caption, which it was
// sent with, when it has one, and what it is, such as a photo which was
// compressed
func describe(caption, description string) string {
	caption = strings.Join(strings.Fields(caption), " ")
	switch {
	case caption == "":
		return description
	case description == "":
		return caption
	case strings.ContainsAny(caption[len(caption)-1:], ".!?"):
		return caption + " " + description
	}
	return caption + ". " + description
}

// compressedPhoto is the description of a photo, which Telegram compressed
const compressedPhoto = "A photo, which Telegram compressed, so small text in it can be hard to read; sent as a file, rather than a photo, it's full size"

// attachments returns the files sent with the message: its photo, as its
// largest size, a document, a voice message, audio, a video, or a video
// message, with their names, and types, or ones for their kind, when they
// have none
func attachments(m *message) []attachment {
	var result []attachment
	if len(m.Photo) > 0 {
		largest := m.Photo[0]
		for _, size := range m.Photo[1:] {
			if size.Width*size.Height > largest.Width*largest.Height {
				largest = size
			}
		}
		result = append(result, attachment{fileID: largest.FileID, Name: "photo-" + largest.FileUniqueID + ".jpg", Type: "image/jpeg", Size: largest.FileSize, Description: compressedPhoto})
	}
	for _, item := range []struct {
		media *media
		name  string
		kind  string
	}{
		{m.Document, "document", "application/octet-stream"},
		{m.Voice, "voice", "audio/ogg"},
		{m.Audio, "audio", "audio/mpeg"},
		{m.Video, "video", "video/mp4"},
		{m.VideoNote, "video-note", "video/mp4"},
	} {
		if item.media == nil || item.media.FileID == "" {
			continue
		}
		// A document's type, when Telegram doesn't know it, is empty, so
		// it's detected from its content, and name, rather than guessed
		mimeType := strings.TrimSpace(item.media.MimeType)
		if mimeType == "application/octet-stream" {
			mimeType = ""
		}
		if mimeType == "" && item.media != m.Document {
			mimeType = item.kind
		}
		name := strings.TrimSpace(item.media.FileName)
		if name == "" {
			name = item.name + "-" + item.media.FileUniqueID
			if extensions, _ := mime.ExtensionsByType(mimeType); len(extensions) > 0 {
				name += preferredExtension(mimeType, extensions)
			}
		}
		result = append(result, attachment{fileID: item.media.FileID, Name: name, Type: mimeType, Size: item.media.FileSize})
	}
	return result
}

// preferredExtension returns the extension of the type, such as .ogg for
// audio/ogg, rather than the first the system knows, which can be unusual
func preferredExtension(mimeType string, extensions []string) string {
	preferred := map[string]string{"audio/ogg": ".ogg", "audio/mpeg": ".mp3", "video/mp4": ".mp4", "image/jpeg": ".jpg"}
	if ext, ok := preferred[mimeType]; ok {
		return ext
	}
	return extensions[0]
}

// opener returns the function which opens the file with the identifier,
// which is downloaded as it's read
func (t *Telegram) opener(fileID string) func(context.Context) (io.ReadCloser, error) {
	return func(ctx context.Context) (io.ReadCloser, error) {
		f, err := call[file](ctx, t.client, "getFile", getFileRequest{FileID: fileID})
		if err != nil {
			return nil, err
		} else if f.FilePath == "" {
			return nil, llm.ErrNotFound.Withf("telegram file %q", fileID)
		}
		r, w := io.Pipe()
		go func() {
			w.CloseWithError(download(ctx, t.client, f.FilePath, func(body io.Reader) error {
				_, err := io.Copy(w, body)
				return err
			}))
		}()
		return r, nil
	}
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
