package webhook

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"uuid"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	toolkit "github.com/mutablelogic/go-llm/toolkit"
	httpclient "github.com/mutablelogic/go-llm/webhook/httpclient"
	httphandler "github.com/mutablelogic/go-llm/webhook/httphandler"
	registry "github.com/mutablelogic/go-llm/webhook/registry"
	schema "github.com/mutablelogic/go-llm/webhook/schema"
	store "github.com/mutablelogic/go-llm/webhook/store/sqlite"
	broadcaster "github.com/mutablelogic/go-server/broadcaster"
	changehandler "github.com/mutablelogic/go-server/broadcaster/httphandler"
	eventschema "github.com/mutablelogic/go-server/broadcaster/schema"
	httprouter "github.com/mutablelogic/go-server/pkg/httprouter"
	sqlite "github.com/mutablelogic/go-sqlite"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

///////////////////////////////////////////////////////////////////////////////
// HELPERS

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

func (l *logs) reset() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.buf.Reset()
}

func (l *logs) count(text string) int {
	return strings.Count(l.String(), text)
}

// server is a webhook server, whose API can be made to fail
type server struct {
	*httptest.Server
	store       *store.Store
	registry    *registry.Registry
	broadcaster *broadcaster.Broadcaster
	endpoint    string       // Its public endpoint, or empty, when it has none
	down        atomic.Bool  // Whether every request fails
	fail        atomic.Int32 // How many more requests to mark calls as seen fail
}

func newServer(t *testing.T) *server {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	b := broadcaster.New()
	go b.Run(ctx)
	require.Eventually(t, b.Running, time.Second, time.Millisecond)
	st, err := store.New(ctx, sqlite.Memory, store.WithBroadcaster(b))
	require.NoError(t, err)
	t.Cleanup(func() { st.Close() })
	r, err := registry.New(st, registry.WithBroadcaster(b))
	require.NoError(t, err)
	router, err := httprouter.NewRouter(ctx, http.NewServeMux(), "/api", "*", "test", "v1")
	require.NoError(t, err)
	self := &server{store: st, registry: r, broadcaster: b}
	require.NoError(t, httphandler.RegisterHandlers(r, router, func() string { return self.endpoint }))
	require.NoError(t, changehandler.RegisterHandlers(b, router))
	self.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if self.down.Load() || (req.Method == http.MethodPatch && req.URL.Path == "/api/webhook/call" && self.fail.Add(-1) >= 0) {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		router.ServeHTTP(w, req)
	}))
	t.Cleanup(func() {
		self.CloseClientConnections()
		self.Close()
	})
	return self
}

// webhook creates a webhook, whose calls are verified, when it has a secret
func (s *server) webhook(t *testing.T, secret string) schema.WebhookToken {
	t.Helper()
	w, err := s.registry.CreateWebhook(t.Context(), schema.WebhookMeta{Session: uuid.New(), Description: "Test webhook", Secret: secret})
	require.NoError(t, err)
	return w
}

// call calls the webhook, with the body, as its service does
func (s *server) call(t *testing.T, w schema.WebhookToken, body string) {
	t.Helper()
	_, _ = s.registry.Call(t.Context(), w.Token, newRequest(body))
}

// newRequest returns a request, as a service calls a webhook, with the body
func newRequest(body string) *http.Request {
	return httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
}

// run runs a namespace of the server, which handles calls with the function,
// until the test ends, and returns what it logs
func (s *server) run(t *testing.T, fn HandleFunc) *logs {
	t.Helper()
	ns, err := New(s.URL + "/api")
	require.NoError(t, err)
	ns.Handle(fn)
	var out logs
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- ns.Run(ctx, slog.New(slog.NewTextHandler(&out, &slog.HandlerOptions{Level: slog.LevelDebug})))
	}()
	t.Cleanup(func() {
		cancel()
		select {
		case err := <-done:
			assert.NoError(t, err)
		case <-time.After(5 * time.Second):
			t.Error("Run didn't return")
		}
	})
	return &out
}

// client returns a client of the server
func (s *server) client(t *testing.T) *httpclient.Client {
	t.Helper()
	c, err := httpclient.New(s.URL + "/api")
	require.NoError(t, err)
	return c
}

// discard is a logger which logs nothing
var discard = slog.New(slog.DiscardHandler)

// pending returns the serial numbers of the calls which are pending
func (s *server) pending(t *testing.T) []uint64 {
	t.Helper()
	list, err := s.registry.ListCalls(t.Context(), schema.CallListRequest{Status: schema.CallPending})
	require.NoError(t, err)
	ids := []uint64{}
	for _, call := range list.Body {
		ids = append(ids, call.ID)
	}
	return ids
}

func eventually(t *testing.T, fn func() bool) {
	t.Helper()
	require.Eventually(t, fn, 5*time.Second, 5*time.Millisecond)
}

func TestMain(m *testing.M) {
	retryMin, retryMax = 20*time.Millisecond, 100*time.Millisecond
	collectWait = 50 * time.Millisecond
	m.Run()
}

///////////////////////////////////////////////////////////////////////////////
// TESTS

func Test_Namespace(t *testing.T) {
	_, err := New(" ")
	assert.ErrorIs(t, err, llm.ErrBadParameter)
	_, err = New("not a url")
	assert.Error(t, err)

	// It's added to a toolkit, without the server
	ns, err := New("http://localhost:1/api")
	require.NoError(t, err)
	tk, err := toolkit.New(t.Context())
	require.NoError(t, err)
	defer tk.Close()
	require.NoError(t, tk.Add(ns))
	added, exists := tk.Namespace(Name)
	assert.True(t, exists)
	assert.Same(t, ns, added)
	_, exists = tk.Namespace("other")
	assert.False(t, exists)
	assert.Equal(t, "webhook", ns.Name())
	assert.NotEmpty(t, ns.Description())
	require.Len(t, ns.Tools(), 6)
	assert.Same(t, ns.Tools()[0], ns.Tools()[0], "the same values, so a toolkit can tell which changed")
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	assert.NoError(t, ns.Watch(ctx, make(chan eventschema.Change, 1)))
	assert.NoError(t, ns.Run(ctx, nil))
}
