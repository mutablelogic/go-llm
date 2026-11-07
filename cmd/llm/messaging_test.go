//go:build !client

package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
	"uuid"

	// Packages
	messaging "github.com/mutablelogic/go-llm/messaging"
	schema "github.com/mutablelogic/go-llm/messaging/schema"
	registryschema "github.com/mutablelogic/go-llm/registry/schema"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

func Test_ChannelStore(t *testing.T) {
	ctx := t.Context()
	path := filepath.Join(t.TempDir(), channelsFile)
	file, err := newJSONFile(path)
	require.NoError(t, err)
	store := &channelStore{file: file}

	// None, until they're saved
	channels, err := store.Load(ctx)
	require.NoError(t, err)
	assert.Empty(t, channels)

	// They're kept, as they were
	saved := []schema.Channel{{
		ID:          uuid.New(),
		ChannelMeta: schema.ChannelMeta{Session: uuid.New(), Provider: "telegram", Address: "-100123", Title: "Family"},
		Created:     time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC),
	}}
	require.NoError(t, store.Save(ctx, saved))
	channels, err = (&channelStore{file: file}).Load(ctx)
	require.NoError(t, err)
	assert.Equal(t, saved, channels)
	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())

	// None, once they're all gone
	require.NoError(t, store.Save(ctx, nil))
	channels, err = store.Load(ctx)
	require.NoError(t, err)
	assert.Empty(t, channels)
}

// provider is a messaging provider which keeps what it's sent, and whose
// chats are connected by the test
type provider struct {
	incoming chan messaging.Message
	sent     chan string
}

func (*provider) Name() string            { return "fake" }
func (*provider) Link(code string) string { return "https://example.com/?start=" + code }
func (p *provider) Receive(ctx context.Context, fn func(context.Context, messaging.Message) error) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case m := <-p.incoming:
			_ = fn(ctx, m)
		}
	}
}
func (p *provider) Send(_ context.Context, _, text string) error {
	p.sent <- text
	return nil
}

func Test_DeliverEvent(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	p := &provider{incoming: make(chan messaging.Message), sent: make(chan string, 8)}
	m, err := messaging.New(messaging.NewStore(), messaging.WithProvider(p))
	require.NoError(t, err)
	go m.Run(ctx, nil)

	// A chat, connected to the session
	session := uuid.New()
	link, _, err := m.Connect(ctx, session, "")
	require.NoError(t, err)
	code := link[len("https://example.com/?start="):]
	p.incoming <- messaging.Message{Address: "1", Code: code}
	<-p.sent

	event := func(status registryschema.EventStatus, message string) *registryschema.Event {
		return &registryschema.Event{EventMeta: registryschema.EventMeta{Message: message, Status: status}}
	}

	// Only what's to be told is sent, and only to the session's channels
	sent, err := deliverEvent(ctx, m, session, event(registryschema.EventPending, "Time to call Anne"))
	require.NoError(t, err)
	assert.True(t, sent)
	assert.Equal(t, "Time to call Anne", <-p.sent)
	for _, e := range []*registryschema.Event{nil, event(registryschema.EventIgnored, "Nothing new"), event(registryschema.EventFailed, "model not found"), event(registryschema.EventPending, " ")} {
		sent, err := deliverEvent(ctx, m, session, e)
		require.NoError(t, err)
		assert.False(t, sent)
	}
	sent, err = deliverEvent(ctx, m, uuid.New(), event(registryschema.EventPending, "Someone else's"))
	require.NoError(t, err)
	assert.False(t, sent)
	sent, err = deliverEvent(ctx, nil, session, event(registryschema.EventPending, "No messaging"))
	require.NoError(t, err)
	assert.False(t, sent)
	assert.Empty(t, p.sent)
}
