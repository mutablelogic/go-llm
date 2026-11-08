package telegram

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	// Packages
	client "github.com/mutablelogic/go-client"
	messaging "github.com/mutablelogic/go-llm/messaging"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

const token = "123456:SECRET-token"

// bot is a fake Bot API, which has the updates, and keeps what's sent
type bot struct {
	mu      sync.Mutex
	updates []update
	sent    []sendMessageRequest
	uploads []uploaded
	paths   []string
	ids     int64
}

// uploaded is a file which was sent, with the method, and field, it was sent
// with
type uploaded struct {
	method, field, name, content, chatID, caption, parseMode string
}

func (b *bot) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	b.mu.Lock()
	b.paths = append(b.paths, r.URL.Path)
	b.mu.Unlock()
	method, ok := strings.CutPrefix(r.URL.Path, "/bot"+token+"/")
	w.Header().Set("Content-Type", "application/json")
	if !ok {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"ok":false,"error_code":401,"description":"Unauthorized"}`))
		return
	}
	switch method {
	case "getMe":
		_ = json.NewEncoder(w).Encode(response[user]{OK: true, Result: user{ID: 1, IsBot: true, FirstName: "Ada", Username: "ada_bot"}})
	case "getUpdates":
		var req getUpdatesRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		b.mu.Lock()
		var result []update
		for _, u := range b.updates {
			if u.ID >= req.Offset {
				result = append(result, u)
			}
		}
		b.mu.Unlock()
		if len(result) == 0 {
			// Wait, as a long poll does, for a while
			select {
			case <-r.Context().Done():
			case <-time.After(50 * time.Millisecond):
			}
		}
		_ = json.NewEncoder(w).Encode(response[[]update]{OK: true, Result: result})
	case "sendMessage":
		var req sendMessageRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		if strings.Contains(req.Text, "unparseable") && req.ParseMode == "HTML" {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"ok":false,"error_code":400,"description":"Bad Request: can't parse entities: Unsupported start tag"}`))
			return
		}
		if req.ChatID == "404" {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"ok":false,"error_code":400,"description":"Bad Request: chat not found"}`))
			return
		}
		b.mu.Lock()
		b.sent = append(b.sent, req)
		b.ids++
		id := b.ids
		b.mu.Unlock()
		_ = json.NewEncoder(w).Encode(response[message]{OK: true, Result: message{ID: id}})
	case "sendPhoto", "sendVoice", "sendAudio", "sendDocument":
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		u := uploaded{method: method, chatID: r.FormValue("chat_id"), caption: r.FormValue("caption"), parseMode: r.FormValue("parse_mode")}
		if strings.Contains(u.caption, "unparseable") && u.parseMode == "HTML" {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"ok":false,"error_code":400,"description":"Bad Request: can't parse entities: Unsupported start tag"}`))
			return
		}
		for field, files := range r.MultipartForm.File {
			u.field, u.name = field, files[0].Filename
			f, _ := files[0].Open()
			data, _ := io.ReadAll(f)
			f.Close()
			u.content = string(data)
		}
		b.mu.Lock()
		b.uploads = append(b.uploads, u)
		b.ids++
		id := b.ids
		b.mu.Unlock()
		_ = json.NewEncoder(w).Encode(response[message]{OK: true, Result: message{ID: id}})
	case "sendChatAction":
		_ = json.NewEncoder(w).Encode(response[bool]{OK: true, Result: true})
	default:
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"ok":false,"error_code":404,"description":"Not Found"}`))
	}
}

func newBot(t *testing.T, b *bot) *Telegram {
	t.Helper()
	server := httptest.NewServer(b)
	t.Cleanup(server.Close)
	tg, err := New(context.Background(), token, client.OptEndpoint(server.URL))
	require.NoError(t, err)
	return tg
}

func Test_Telegram_New(t *testing.T) {
	b := &bot{}
	tg := newBot(t, b)
	assert.Equal(t, "ada_bot", tg.Username())
	assert.Equal(t, "telegram", tg.Name())
	assert.Equal(t, "https://t.me/ada_bot?start=abc-_1", tg.Link("abc-_1"))
	assert.Equal(t, []string{"/bot" + token + "/getMe"}, b.paths)

	// A wrong token is an error, which doesn't have it
	server := httptest.NewServer(b)
	defer server.Close()
	_, err := New(context.Background(), "999:WRONG", client.OptEndpoint(server.URL))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Unauthorized")
	assert.NotContains(t, err.Error(), "WRONG")

	_, err = New(context.Background(), " ")
	assert.Error(t, err)
}

func Test_Telegram_Receive(t *testing.T) {
	b := &bot{updates: []update{
		{ID: 10, Message: &message{ID: 1, From: &user{ID: 7, FirstName: "David", LastName: "T"}, Chat: chat{ID: 7, Type: "private", FirstName: "David", LastName: "T"}, Date: 1700000000, Text: "/start abc"}},
		{ID: 11, Message: &message{ID: 2, From: &user{ID: 8, IsBot: true}, Chat: chat{ID: 8}, Text: "from a bot"}},
		{ID: 12, Message: &message{ID: 3, From: &user{ID: 7, FirstName: "David"}, Chat: chat{ID: -100, Type: "group", Title: "Family"}, Text: "hello"}},
		{ID: 13},
	}}
	tg := newBot(t, b)

	ctx, cancel := context.WithCancel(context.Background())
	var got []messaging.Message
	err := tg.Receive(ctx, func(_ context.Context, m messaging.Message) error {
		got = append(got, m)
		if len(got) == 2 {
			cancel()
		}
		return nil
	})
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, messaging.Message{Address: "7", Title: "David T", From: "David T", Code: "abc", Text: "/start abc", Time: time.Unix(1700000000, 0).UTC()}, got[0])
	assert.Equal(t, "-100", got[1].Address)
	assert.Equal(t, "Family", got[1].Title)
	assert.Empty(t, got[1].Code)
	assert.Equal(t, int64(14), tg.offset)
}

func Test_Telegram_Send(t *testing.T) {
	b := &bot{}
	tg := newBot(t, b)
	ctx := context.Background()

	_, err := tg.Send(ctx, "7", "Hello **David**")
	require.NoError(t, err)
	require.NoError(t, tg.Typing(ctx, "7"))
	assert.Equal(t, sendMessageRequest{ChatID: "7", Text: "Hello <b>David</b>", ParseMode: "HTML", LinkPreviewOptions: &linkPreviewOptions{IsDisabled: true}}, b.sent[0])

	// A long reply is in parts, between paragraphs
	long := strings.Repeat("word ", 700) + "\n\n" + strings.Repeat("more ", 700)
	sent, err := tg.Send(ctx, "7", long)
	require.NoError(t, err)
	require.Len(t, sent, 2, "a message for each part")
	assert.True(t, sent[0].Text)
	assert.NotEqual(t, sent[0].ID, sent[1].ID)
	require.Len(t, b.sent, 3)
	assert.Equal(t, strings.TrimSpace(strings.Repeat("word ", 700)), b.sent[1].Text)
	assert.Equal(t, strings.TrimSpace(strings.Repeat("more ", 700)), b.sent[2].Text)

	// HTML which Telegram can't parse is sent as plain text
	b.mu.Lock()
	b.sent = nil
	b.mu.Unlock()
	_, err = tg.Send(ctx, "7", "**Bold** <unparseable>")
	require.NoError(t, err)
	require.Len(t, b.sent, 1)
	assert.Equal(t, "Bold <unparseable>", b.sent[0].Text)
	assert.Empty(t, b.sent[0].ParseMode)

	_, err = tg.Send(ctx, "404", "Hello")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "chat not found")
	assert.NotContains(t, err.Error(), "SECRET")
}

func Test_Telegram_StartCode(t *testing.T) {
	for text, want := range map[string]string{
		"/start abc":         "abc",
		"/start@ada_bot abc": "abc",
		"/start":             "",
		"/stop abc":          "",
		"hello":              "",
		"/start a b":         "",
	} {
		assert.Equal(t, want, startCode(text), text)
	}
}

func Test_Telegram_Split(t *testing.T) {
	assert.Equal(t, []string{"short"}, split("short", 10))
	assert.Equal(t, []string{"one two", "three"}, split("one two three", 10))
	assert.Equal(t, []string{"para one.", "para two"}, split("para one.\n\npara two", 15))
	assert.Equal(t, []string{"abcdefghij", "klm"}, split("abcdefghijklm", 10))
	assert.Equal(t, []string{"ééééé", "ééé"}, split("éééééééé", 5))
	assert.Empty(t, split("  ", 10))
}

func Test_Telegram_RedirectError(t *testing.T) {
	// A redirect with a Location which isn't valid is an error, which hasn't
	// the token
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Location", "http://[::1]:namedport")
		w.WriteHeader(http.StatusFound)
	}))
	defer server.Close()
	_, err := New(context.Background(), token, client.OptEndpoint(server.URL))
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "SECRET")
}

func Test_Telegram_SendFile(t *testing.T) {
	b := &bot{}
	tg := newBot(t, b)
	ctx := context.Background()
	opened := 0
	file := func(name, mimeType, content string) (messaging.File, func() (io.ReadCloser, error)) {
		return messaging.File{URI: "resource:///x", Name: name, Type: mimeType, Size: int64(len(content))}, func() (io.ReadCloser, error) {
			opened++
			return io.NopCloser(strings.NewReader(content)), nil
		}
	}

	// An image is a photo, with its caption, formatted
	f, r := file("chart.png", "image/png", "PNG")
	sent, err := tg.SendFile(ctx, "7", f, r, "The **weather**")
	require.NoError(t, err)
	assert.Equal(t, []messaging.Sent{{ID: "1", File: "chart.png", Caption: true}}, sent)
	// Ogg is a voice message, MP3 a track, and WAV a document
	f, r = file("speech.ogg", "audio/ogg", "OGG")
	_, err = tg.SendFile(ctx, "7", f, r, "")
	require.NoError(t, err)
	f, r = file("speech.mp3", "audio/mpeg", "MP3")
	_, err = tg.SendFile(ctx, "7", f, r, "")
	require.NoError(t, err)
	f, r = file("", "audio/wav", "RIFF")
	_, err = tg.SendFile(ctx, "7", f, r, "")
	require.NoError(t, err)
	require.Len(t, b.uploads, 4)
	assert.Equal(t, uploaded{method: "sendPhoto", field: "photo", name: "chart.png", content: "PNG", chatID: "7", caption: "The <b>weather</b>", parseMode: "HTML"}, b.uploads[0])
	assert.Equal(t, uploaded{method: "sendVoice", field: "voice", name: "speech.ogg", content: "OGG", chatID: "7"}, b.uploads[1])
	assert.Equal(t, uploaded{method: "sendAudio", field: "audio", name: "speech.mp3", content: "MP3", chatID: "7"}, b.uploads[2])
	assert.Equal(t, "sendDocument", b.uploads[3].method)
	assert.Equal(t, "document", b.uploads[3].field)
	assert.Equal(t, "file.wav", b.uploads[3].name)
	assert.Empty(t, b.sent)

	// A caption which is too long is a message before it
	long := strings.Repeat("word ", 300)
	f, r = file("chart.png", "image/png", "PNG")
	sent, err = tg.SendFile(ctx, "7", f, r, long)
	require.NoError(t, err)
	assert.Equal(t, []messaging.Sent{{ID: "5", Text: true}, {ID: "6", File: "chart.png"}}, sent, "the caption, and then the file, without it")
	require.Len(t, b.sent, 1)
	assert.Equal(t, strings.TrimSpace(long), b.sent[0].Text)
	assert.Empty(t, b.uploads[4].caption)

	// A file which is too large is a note, after its caption, which isn't
	// lost, and it isn't opened
	opened = 0
	f = messaging.File{Name: "photo.png", Type: "image/png", Size: 60 << 20}
	sent, err = tg.SendFile(ctx, "7", f, func() (io.ReadCloser, error) {
		opened++
		return io.NopCloser(strings.NewReader("")), nil
	}, "A big photo")
	require.NoError(t, err)
	assert.Equal(t, []messaging.Sent{{ID: "7", Text: true}, {ID: "8", File: "photo.png", Note: "too large to send, so this is a note in its place"}}, sent)
	assert.Len(t, b.uploads, 5)
	assert.Zero(t, opened)
	assert.Equal(t, "A big photo", b.sent[1].Text)
	assert.Equal(t, "📎 photo.png is too large to send here (60.0 MB)", b.sent[2].Text)

	// A caption whose formatting Telegram can't parse is sent as plain
	// text, with the file, opened again
	opened = 0
	f, r = file("chart.png", "image/png", "PNG")
	sent, err = tg.SendFile(ctx, "7", f, r, "**unparseable** chart")
	require.NoError(t, err)
	assert.Equal(t, []messaging.Sent{{ID: "9", File: "chart.png", Caption: true}}, sent)
	assert.Equal(t, 2, opened)
	require.Len(t, b.uploads, 6)
	assert.Equal(t, uploaded{method: "sendPhoto", field: "photo", name: "chart.png", content: "PNG", chatID: "7", caption: "unparseable chart"}, b.uploads[5])
}

func Test_Telegram_Kind(t *testing.T) {
	for _, c := range []struct {
		mimeType string
		size     int64
		want     string
	}{
		{"image/jpeg", 100, "photo"},
		{"image/png; charset=binary", 100, "photo"},
		{"image/png", 11 << 20, "document"},
		{"image/gif", 100, "document"},
		{"audio/ogg", 100, "voice"},
		{"audio/mpeg", 100, "audio"},
		{"audio/x-m4a", 100, "audio"},
		{"audio/wav", 100, "document"},
		{"text/markdown", 100, "document"},
		{"", 100, "document"},
	} {
		assert.Equal(t, c.want, kind(messaging.File{Type: c.mimeType, Size: c.size}), c.mimeType)
	}
}
