package telegram

import (
	"context"
	"io"
	"net/http"
	"strings"

	// Packages
	client "github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// response is the Bot API's response to a method
type response[T any] struct {
	OK          bool   `json:"ok"`
	Result      T      `json:"result"`
	ErrorCode   int    `json:"error_code,omitempty"`
	Description string `json:"description,omitempty"`
}

// user is a Telegram user, or bot
type user struct {
	ID        int64  `json:"id"`
	IsBot     bool   `json:"is_bot"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name,omitempty"`
	Username  string `json:"username,omitempty"`
}

// chat is a Telegram chat: private, with a person, or a group
type chat struct {
	ID        int64  `json:"id"`
	Type      string `json:"type"`
	Title     string `json:"title,omitempty"`
	FirstName string `json:"first_name,omitempty"`
	LastName  string `json:"last_name,omitempty"`
	Username  string `json:"username,omitempty"`
}

// message is a message sent to the bot, with its text, or the caption of
// what's sent with it, such as photos, a document, or a voice message, and
// the album it's in, when it's one of several photos, or videos, sent
// together
type message struct {
	ID           int64       `json:"message_id"`
	From         *user       `json:"from,omitempty"`
	Chat         chat        `json:"chat"`
	Date         int64       `json:"date"`
	Text         string      `json:"text,omitempty"`
	Caption      string      `json:"caption,omitempty"`
	MediaGroupID string      `json:"media_group_id,omitempty"`
	Photo        []photoSize `json:"photo,omitempty"`
	Document     *media      `json:"document,omitempty"`
	Voice        *media      `json:"voice,omitempty"`
	Audio        *media      `json:"audio,omitempty"`
	Video        *media      `json:"video,omitempty"`
	VideoNote    *media      `json:"video_note,omitempty"`
}

// photoSize is one of the sizes of a photo
type photoSize struct {
	FileID       string `json:"file_id"`
	FileUniqueID string `json:"file_unique_id"`
	Width        int    `json:"width"`
	Height       int    `json:"height"`
	FileSize     int64  `json:"file_size,omitempty"`
}

// media is a file sent with a message, such as a document, or a voice
// message, with its name, and type, when they're known
type media struct {
	FileID       string `json:"file_id"`
	FileUniqueID string `json:"file_unique_id"`
	FileName     string `json:"file_name,omitempty"`
	MimeType     string `json:"mime_type,omitempty"`
	FileSize     int64  `json:"file_size,omitempty"`
	Duration     int    `json:"duration,omitempty"`
}

// file is a file which can be downloaded, by its path
type file struct {
	FileID   string `json:"file_id"`
	FileSize int64  `json:"file_size,omitempty"`
	FilePath string `json:"file_path,omitempty"`
}

// getFileRequest asks for the path of the file, to download it
type getFileRequest struct {
	FileID string `json:"file_id"`
}

// update is what happened, which the bot is told: here, a message
type update struct {
	ID      int64    `json:"update_id"`
	Message *message `json:"message,omitempty"`
}

// getUpdatesRequest asks for the updates after the offset, waiting up to
// the timeout, in seconds, for one
type getUpdatesRequest struct {
	Offset         int64    `json:"offset,omitempty"`
	Timeout        int      `json:"timeout"`
	AllowedUpdates []string `json:"allowed_updates"`
}

// sendMessageRequest sends the text to the chat, as HTML, when it has a
// parse mode
type sendMessageRequest struct {
	ChatID             string              `json:"chat_id"`
	Text               string              `json:"text"`
	ParseMode          string              `json:"parse_mode,omitempty"`
	LinkPreviewOptions *linkPreviewOptions `json:"link_preview_options,omitempty"`
}

// linkPreviewOptions say whether a message's links have previews
type linkPreviewOptions struct {
	IsDisabled bool `json:"is_disabled"`
}

// sendChatActionRequest shows the action, such as typing, in the chat
type sendChatActionRequest struct {
	ChatID string `json:"chat_id"`
	Action string `json:"action"`
}

// sendFileRequest sends a file to the chat, with its caption, in the field
// of its method, such as photo, which are sent as multipart form data
type sendPhotoRequest struct {
	ChatID    string     `json:"chat_id"`
	Photo     types.File `json:"photo"`
	Caption   string     `json:"caption,omitempty"`
	ParseMode string     `json:"parse_mode,omitempty"`
}

type sendVoiceRequest struct {
	ChatID    string     `json:"chat_id"`
	Voice     types.File `json:"voice"`
	Caption   string     `json:"caption,omitempty"`
	ParseMode string     `json:"parse_mode,omitempty"`
}

type sendAudioRequest struct {
	ChatID    string     `json:"chat_id"`
	Audio     types.File `json:"audio"`
	Caption   string     `json:"caption,omitempty"`
	ParseMode string     `json:"parse_mode,omitempty"`
}

type sendDocumentRequest struct {
	ChatID    string     `json:"chat_id"`
	Document  types.File `json:"document"`
	Caption   string     `json:"caption,omitempty"`
	ParseMode string     `json:"parse_mode,omitempty"`
}

// tokenTransport adds the bot's token to the path of each request, as the
// Bot API has it, as /bot<token>/<method>, or, to download a file,
// /file/bot<token>/<path>, beneath the client's tracing and logging, so
// they, and the errors of requests, have the path without it
type tokenTransport struct {
	token string
	base  http.RoundTripper
}

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

// endpoint is the Bot API's
const endpoint = "https://api.telegram.org"

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// newClient returns a client of the Bot API, with the bot's token, and the
// options, whose endpoint can be replaced, such as for tests
func newClient(token string, opts ...client.ClientOpt) (*client.Client, error) {
	if strings.TrimSpace(token) == "" {
		return nil, llm.ErrBadParameter.With("missing Telegram bot token")
	}
	// The token's transport is the first option, so it's beneath the others,
	// such as the tracing
	opts = append([]client.ClientOpt{withToken(token), client.OptEndpoint(endpoint)}, opts...)
	return client.New(opts...)
}

// withToken is the option which adds the token's transport, beneath the
// transport the client has, when it's applied, which is the first option,
// so the tracing, and logging, which other options add, are above it.
// Options which need the client's transport to be an *http.Transport, such
// as client.OptSkipVerify, can't be used, as it isn't one.
func withToken(token string) client.ClientOpt {
	return func(c *client.Client) error {
		base := c.Client.Transport
		if base == nil {
			base = http.DefaultTransport
		}
		c.Client.Transport = &tokenTransport{token: token, base: base}
		return nil
	}
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// RoundTrip sends a copy of the request, whose path has the token, and
// returns the response, with the request, rather than its copy, so what
// reads the response's request, such as an error of a redirect, doesn't
// have the token
func (t *tokenTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	clone := req.Clone(req.Context())
	if path, ok := strings.CutPrefix(req.URL.Path, "/file/"); ok {
		clone.URL.Path = "/file/bot" + t.token + "/" + path
	} else {
		clone.URL.Path = "/bot" + t.token + "/" + strings.TrimPrefix(req.URL.Path, "/")
	}
	clone.URL.RawPath = ""
	resp, err := t.base.RoundTrip(clone)
	if resp != nil {
		resp.Request = req
	}
	return resp, err
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// upload calls the Bot API's method, with the request, which has a file,
// as multipart form data, which is read as it's sent, and returns its
// result, or its error
func upload[T any](ctx context.Context, c *client.Client, method string, req any) (T, error) {
	var result response[T]
	payload, err := client.NewStreamingMultipartRequest(req, types.ContentTypeJSON)
	if err != nil {
		return result.Result, err
	}
	if err := c.DoWithContext(ctx, payload, &result, client.OptPath(method), client.OptNoTimeout()); err != nil {
		return result.Result, err
	} else if !result.OK {
		return result.Result, llm.ErrInternalServerError.Withf("telegram %s: %s", method, result.Description)
	}
	return result.Result, nil
}

// call calls the Bot API's method, with the request, and returns its result,
// or its error
func call[T any](ctx context.Context, c *client.Client, method string, req any, opts ...client.RequestOpt) (T, error) {
	var result response[T]
	payload, err := client.NewJSONRequest(req)
	if err != nil {
		return result.Result, err
	}
	opts = append([]client.RequestOpt{client.OptPath(method)}, opts...)
	if err := c.DoWithContext(ctx, payload, &result, opts...); err != nil {
		return result.Result, err
	} else if !result.OK {
		return result.Result, llm.ErrInternalServerError.Withf("telegram %s: %s", method, result.Description)
	}
	return result.Result, nil
}

// download downloads the file at the path, which getFile returned, and
// calls the function with its content, as it's read
func download(ctx context.Context, c *client.Client, path string, fn func(io.Reader) error) error {
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	elems := make([]any, 0, len(parts)+1)
	elems = append(elems, "file")
	for _, part := range parts {
		elems = append(elems, part)
	}
	return c.DoWithContext(ctx, client.NewRequest(), &body{fn: fn}, client.OptPath(elems...), client.OptNoTimeout())
}

// body is a response's body, which is read by a function, as it's received
type body struct {
	fn func(io.Reader) error
}

func (b *body) Unmarshal(_ http.Header, r io.Reader) error {
	return b.fn(r)
}
