package telegram

import (
	"context"
	"encoding/json"
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
	paths   []string
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
		b.mu.Unlock()
		_ = json.NewEncoder(w).Encode(response[message]{OK: true, Result: message{ID: 1}})
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

	require.NoError(t, tg.Send(ctx, "7", "Hello **David**"))
	require.NoError(t, tg.Typing(ctx, "7"))
	assert.Equal(t, sendMessageRequest{ChatID: "7", Text: "Hello <b>David</b>", ParseMode: "HTML", LinkPreviewOptions: &linkPreviewOptions{IsDisabled: true}}, b.sent[0])

	// A long reply is in parts, between paragraphs
	long := strings.Repeat("word ", 700) + "\n\n" + strings.Repeat("more ", 700)
	require.NoError(t, tg.Send(ctx, "7", long))
	require.Len(t, b.sent, 3)
	assert.Equal(t, strings.TrimSpace(strings.Repeat("word ", 700)), b.sent[1].Text)
	assert.Equal(t, strings.TrimSpace(strings.Repeat("more ", 700)), b.sent[2].Text)

	// HTML which Telegram can't parse is sent as plain text
	b.mu.Lock()
	b.sent = nil
	b.mu.Unlock()
	require.NoError(t, tg.Send(ctx, "7", "**Bold** <unparseable>"))
	require.Len(t, b.sent, 1)
	assert.Equal(t, "Bold <unparseable>", b.sent[0].Text)
	assert.Empty(t, b.sent[0].ParseMode)

	err := tg.Send(ctx, "404", "Hello")
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
