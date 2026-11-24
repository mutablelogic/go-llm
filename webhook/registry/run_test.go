package registry_test

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"
	"uuid"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	registry "github.com/mutablelogic/go-llm/webhook/registry"
	schema "github.com/mutablelogic/go-llm/webhook/schema"
	store "github.com/mutablelogic/go-llm/webhook/store/sqlite"
	broadcaster "github.com/mutablelogic/go-server/broadcaster"
	eventschema "github.com/mutablelogic/go-server/broadcaster/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
	sqlite "github.com/mutablelogic/go-sqlite"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

// logs is a buffer, which is written to, and read, at once
type logs struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (l *logs) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buf.Write(p)
}

func (l *logs) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buf.String()
}

func Test_Registry_Run(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// A broadcaster, which is running, and a registry, which publishes to it
	b := broadcaster.New()
	go b.Run(ctx)
	require.Eventually(t, b.Running, time.Second, 5*time.Millisecond)
	s, err := store.New(ctx, sqlite.Memory, store.WithBroadcaster(b))
	require.NoError(t, err)
	defer s.Close()
	r, err := registry.New(s, registry.WithBroadcaster(b))
	require.NoError(t, err)

	// Run logs what it watches
	var out logs
	logger := slog.New(slog.NewTextHandler(&out, &slog.HandlerOptions{Level: slog.LevelDebug}))
	runErr := make(chan error, 1)
	go func() { runErr <- r.Run(ctx, logger) }()
	time.Sleep(50 * time.Millisecond)

	// A webhook, which is created, and changed, and its calls, which are
	// accepted, and refused
	hook, err := r.CreateWebhook(ctx, schema.WebhookMeta{Session: uuid.New(), Description: "GitHub", Type: "github", Secret: "s3cret"})
	require.NoError(t, err)
	_, err = r.UpdateWebhook(ctx, hook.ID, schema.WebhookRequest{Description: types.Ptr("GitHub")})
	require.NoError(t, err)
	_, err = r.Call(ctx, hook.Token, githubRequest(`{"action":"opened"}`, "s3cret", "d1"))
	require.NoError(t, err)
	_, err = r.Call(ctx, hook.Token, githubRequest(`{"action":"closed"}`, "wrong", "d2"))
	require.ErrorIs(t, err, llm.ErrForbidden)

	// The calls are read, as they're logged, so they're logged before
	// they're deleted, with their webhook
	require.Eventually(t, func() bool { return strings.Contains(out.String(), "webhook call refused") }, time.Second, 10*time.Millisecond)
	_, err = r.DeleteWebhook(ctx, hook.ID)
	require.NoError(t, err)

	// Changes of other schemas, of tables of the same names, aren't logged
	b.Publish(eventschema.Change{Action: eventschema.ChangeCreate, Schema: "main", Table: "webhook", Row: "99"})

	require.Eventually(t, func() bool { return strings.Contains(out.String(), "webhook deleted") }, time.Second, 10*time.Millisecond)
	require.Eventually(t, func() bool { return strings.Count(out.String(), "webhook call deleted") == 2 }, time.Second, 10*time.Millisecond, "the calls are deleted with their webhook")
	logged := out.String()
	assert.Contains(t, logged, `level=INFO msg="webhook created"`)
	assert.Contains(t, logged, `level=INFO msg="webhook changed"`)
	assert.Contains(t, logged, `level=INFO msg="webhook call accepted"`)
	assert.Contains(t, logged, "event=issues.opened delivery=d1")
	assert.Contains(t, logged, `level=WARN msg="webhook call refused"`)
	assert.Contains(t, logged, "event=issues.closed delivery=d2")
	assert.Contains(t, logged, "X-Hub-Signature-256")
	assert.NotContains(t, logged, hook.Token)
	assert.NotContains(t, logged, "webhook=99")

	// It returns nil, once it's cancelled
	cancel()
	select {
	case err := <-runErr:
		assert.NoError(t, err)
	case <-time.After(time.Second):
		t.Fatal("Run didn't return")
	}
}

func Test_Registry_RunWithoutBroadcaster(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	r, _ := newRegistry(t)
	assert.NoError(t, r.Run(ctx, nil))
	_, err := registry.New(nil, registry.WithBroadcaster(nil))
	assert.ErrorIs(t, err, llm.ErrBadParameter)
}
