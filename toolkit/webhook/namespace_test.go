package webhook

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
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
	fail        atomic.Int32 // How many more requests of a call fail
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
		if self.down.Load() || (strings.HasPrefix(req.URL.Path, "/api/webhook/call/") && self.fail.Add(-1) >= 0) {
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

// run runs a namespace of the server, until the test ends, and returns what
// it logs
func (s *server) run(t *testing.T) *logs {
	t.Helper()
	ns, err := New(s.URL + "/api")
	require.NoError(t, err)
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

func eventually(t *testing.T, fn func() bool) {
	t.Helper()
	require.Eventually(t, fn, 5*time.Second, 5*time.Millisecond)
}

func TestMain(m *testing.M) {
	retryMin, retryMax = 20*time.Millisecond, 100*time.Millisecond
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
	assert.Equal(t, "webhook", ns.Name())
	assert.NotEmpty(t, ns.Description())
	require.Len(t, ns.Tools(), 5)
	assert.Same(t, ns.Tools()[0], ns.Tools()[0], "the same values, so a toolkit can tell which changed")
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	assert.NoError(t, ns.Watch(ctx, make(chan eventschema.Change, 1)))
	assert.NoError(t, ns.Run(ctx, nil))
}

func Test_Run(t *testing.T) {
	s := newServer(t)
	w, signed := s.webhook(t, ""), s.webhook(t, "s3cret")
	s.call(t, w, "before it's watched")
	out := s.run(t)
	eventually(t, func() bool { return out.count("webhook calls are watched") == 1 })

	// A call, which was accepted, and one which was refused, with the webhook
	// and its session, but not what was sent, nor a call before it ran
	s.call(t, w, "what was sent")
	s.call(t, signed, "not signed")
	eventually(t, func() bool { return out.count(`msg="webhook call refused"`) == 1 })
	logged := out.String()
	assert.Equal(t, 1, strings.Count(logged, `level=INFO msg="webhook call" namespace=webhook call=2 webhook=1 description="Test webhook" session=`+w.Session.String()))
	assert.Contains(t, logged, "level=WARN msg=\"webhook call refused\" namespace=webhook call=3 webhook=2")
	assert.Contains(t, logged, "bearer token")
	for _, text := range []string{"what was sent", "not signed", "s3cret", w.Token, signed.Token, "call=1 "} {
		assert.NotContains(t, logged, text)
	}

	// Other changes aren't calls
	_, err := s.registry.DeleteWebhook(t.Context(), signed.ID)
	require.NoError(t, err)
	s.broadcaster.Publish(eventschema.Change{Action: eventschema.ChangeCreate, Schema: "other", Table: "call", Row: "2"})
	s.call(t, w, "x")
	eventually(t, func() bool { return out.count("call=4 ") == 1 })
	assert.Equal(t, 3, out.count("msg=\"webhook call"+`" `)+out.count("webhook call refused"))
}

func Test_RunReconnects(t *testing.T) {
	s := newServer(t)
	w := s.webhook(t, "")
	out := s.run(t)
	eventually(t, func() bool { return out.count("webhook calls are watched") == 1 })
	s.call(t, w, "1")
	eventually(t, func() bool { return out.count("call=1 ") == 1 })

	// The calls which are received while it can't reach the server, more than
	// are read at once, are logged once it can, once each, in order
	s.down.Store(true)
	s.CloseClientConnections()
	eventually(t, func() bool { return out.count("webhook calls aren't watched") > 1 })
	for range 2*missedLimit + 3 {
		s.call(t, w, "missed")
	}
	s.down.Store(false)
	eventually(t, func() bool { return out.count("call=24 ") == 1 })
	s.call(t, w, "25")
	eventually(t, func() bool { return out.count("call=25 ") == 1 })
	logged, at := out.String(), -1
	for id := uint64(1); id <= 25; id++ {
		text := "call=" + strconv.FormatUint(id, 10) + " "
		assert.Equal(t, 1, strings.Count(logged, text), text)
		assert.Greater(t, strings.Index(logged, text), at)
		at = strings.Index(logged, text)
	}
}

func Test_RunCallNotRead(t *testing.T) {
	// The first call, which can't be read at first, isn't lost, nor is
	// another
	s := newServer(t)
	w := s.webhook(t, "")
	out := s.run(t)
	eventually(t, func() bool { return out.count("webhook calls are watched") == 1 })
	s.fail.Store(1)
	s.call(t, w, "1")
	eventually(t, func() bool { return out.count("call=1 ") == 1 })
	s.fail.Store(1)
	s.call(t, w, "2")
	eventually(t, func() bool { return out.count("call=2 ") == 1 })
	assert.Equal(t, 2, out.count("webhook calls aren't watched"))
}

func Test_RunMissedChanges(t *testing.T) {
	// A change without a row says that changes were missed, so calls are
	// listed, rather than waiting for a change of each
	s := newServer(t)
	w := s.webhook(t, "")
	out := s.run(t)
	eventually(t, func() bool { return out.count("webhook calls are watched") == 1 })
	for range 40 {
		s.call(t, w, "x")
	}
	eventually(t, func() bool { return out.count("call=40 ") == 1 })
	assert.Equal(t, 40, out.count(`msg="webhook call" `))
}

func Test_RunServerStops(t *testing.T) {
	// A server which isn't there yet is watched, once it is
	s := newServer(t)
	s.down.Store(true)
	out := s.run(t)
	eventually(t, func() bool { return out.count("webhook calls aren't watched") > 2 })
	assert.Zero(t, out.count("webhook calls are watched"))
	s.down.Store(false)
	eventually(t, func() bool { return out.count("webhook calls are watched") == 1 })
	s.call(t, s.webhook(t, ""), "x")
	eventually(t, func() bool { return out.count("call=1 ") == 1 })
}

func Test_Position(t *testing.T) {
	ctx := t.Context()
	s := newServer(t)
	hook := s.webhook(t, "")
	for range 4 {
		s.call(t, hook, "x")
	}
	newNamespace := func(last uint64) *Namespace {
		ns, err := New(s.URL + "/api")
		require.NoError(t, err)
		ns.seen, ns.last = true, last
		return ns
	}
	created := func(row string) eventschema.Change {
		return eventschema.Change{Action: eventschema.ChangeCreate, Schema: "webhook", Table: "call", Row: row}
	}

	// A call which was missed isn't handled again, when the watch is sent
	// it too, and one after it is
	var out logs
	logger := slog.New(slog.NewTextHandler(&out, nil))
	ns := newNamespace(1)
	require.NoError(t, ns.handleMissed(ctx, logger))
	assert.Equal(t, uint64(4), ns.last)
	require.NoError(t, ns.handleChange(ctx, logger, created("3")))
	require.NoError(t, ns.handleChange(ctx, logger, created("4")))
	assert.Equal(t, 3, out.count(`msg="webhook call" `))
	s.call(t, hook, "x")
	require.NoError(t, ns.handleChange(ctx, logger, created("5")))
	assert.Equal(t, 1, out.count("call=5 "))

	// A server whose serial numbers start again, such as one in memory
	// which restarted, has calls before the last
	ns = newNamespace(50)
	require.NoError(t, ns.handleMissed(ctx, logger))
	require.NoError(t, ns.handleChange(ctx, logger, created("1")))
	assert.Equal(t, uint64(1), ns.last)
	assert.Equal(t, 1, out.count("call=1 "))

	// A call which was deleted is skipped, and what isn't a call is ignored
	require.NoError(t, ns.handleChange(ctx, logger, created("99")))
	assert.Equal(t, uint64(99), ns.last)
	for _, row := range []string{"0", "x", "-1"} {
		require.NoError(t, ns.handleChange(ctx, logger, created(row)))
	}
	assert.Equal(t, uint64(99), ns.last)
}

func Test_RunAgain(t *testing.T) {
	s := newServer(t)
	hook := s.webhook(t, "")
	ns, err := New(s.URL + "/api")
	require.NoError(t, err)
	var out logs
	run := func() (stop func()) {
		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan error, 1)
		go func() { done <- ns.Run(ctx, slog.New(slog.NewTextHandler(&out, nil))) }()
		return func() {
			cancel()
			assert.NoError(t, <-done)
		}
	}

	// It's run once at a time
	stop := run()
	eventually(t, func() bool { return out.count("webhook calls are watched") == 1 })
	assert.ErrorIs(t, ns.Run(t.Context(), nil), llm.ErrConflict)
	s.call(t, hook, "1")
	eventually(t, func() bool { return out.count("call=1 ") == 1 })
	stop()

	// When it's run again, it logs the calls since it last ran
	s.call(t, hook, "2")
	defer run()()
	eventually(t, func() bool { return out.count("call=2 ") == 1 })
	assert.Equal(t, 1, out.count("call=1 "))
}

// runUntil runs the namespace, until the function returns true
func runUntil(t *testing.T, ns *Namespace, out *logs, fn func() bool) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- ns.Run(ctx, slog.New(slog.NewTextHandler(out, nil))) }()
	defer func() {
		cancel()
		assert.NoError(t, <-done)
	}()
	eventually(t, fn)
}

func Test_RunServerRestarts(t *testing.T) {
	for _, test := range []struct {
		name          string
		before, after int // How many calls each server has
	}{
		{"Fewer calls than before", 5, 3},
		{"More calls than before", 2, 5},
		{"As many calls as before", 3, 3},
	} {
		t.Run(test.name, func(t *testing.T) {
			// The calls of a server, which are handled
			first, second := newServer(t), newServer(t)
			hook := first.webhook(t, "")
			ns, err := New(first.URL + "/api")
			require.NoError(t, err)
			var out logs
			called := false
			runUntil(t, ns, &out, func() bool {
				if !called && out.count("webhook calls are watched") == 1 {
					called = true
					for range test.before {
						first.call(t, hook, "x")
					}
				}
				return out.count(`msg="webhook call" `) == test.before
			})

			// And of one which replaced it, whose serial numbers start again,
			// before it's watched, and then while it is
			hook = second.webhook(t, "")
			for range test.after {
				second.call(t, hook, "x")
			}
			ns.client, err = httpclient.New(second.URL + "/api")
			require.NoError(t, err)
			out.reset()
			runUntil(t, ns, &out, func() bool { return out.count(`msg="webhook call" `) == test.after })
			called = false
			runUntil(t, ns, &out, func() bool {
				if !called && out.count("webhook calls are watched") == 2 {
					called = true
					second.call(t, hook, "x")
				}
				return out.count("call="+strconv.Itoa(test.after+1)+" ") == 1
			})
			for id := 1; id <= test.after; id++ {
				assert.Equal(t, 1, out.count("call="+strconv.Itoa(id)+" "), id)
			}
		})
	}
}

func Test_RunServerRestartsNotRead(t *testing.T) {
	// The first call of a server whose serial numbers started again, which
	// can't be read at first, isn't lost, though none was handled before
	s := newServer(t)
	hook := s.webhook(t, "")
	ns, err := New(s.URL + "/api")
	require.NoError(t, err)
	ns.seen, ns.last = true, 50
	var out logs
	s.fail.Store(1)
	called := false
	runUntil(t, ns, &out, func() bool {
		if !called && out.count("webhook calls are watched") == 1 {
			called = true
			s.call(t, hook, "x")
		}
		return out.count("call=1 ") == 1
	})
	assert.Equal(t, 1, out.count("webhook calls aren't watched"))
}

func Test_RunCallsDeleted(t *testing.T) {
	// Calls which were deleted, including the last, don't look like a server
	// which restarted, so those which remain aren't handled again
	s := newServer(t)
	keep, drop := s.webhook(t, ""), s.webhook(t, "")
	ns, err := New(s.URL + "/api")
	require.NoError(t, err)
	var out logs
	called := false
	runUntil(t, ns, &out, func() bool {
		if !called && out.count("webhook calls are watched") == 1 {
			called = true
			s.call(t, drop, "1")
			s.call(t, keep, "2")
			s.call(t, drop, "3")
		}
		return out.count(`msg="webhook call" `) == 3
	})
	_, err = s.registry.DeleteWebhook(t.Context(), drop.ID)
	require.NoError(t, err)
	s.call(t, keep, "4")
	runUntil(t, ns, &out, func() bool { return out.count("call=4 ") == 1 })
	assert.Equal(t, 4, out.count(`msg="webhook call" `))
}

func Test_RunCallNotReadAfterOthers(t *testing.T) {
	// The first call which is seen, which can't be read at first, isn't
	// taken to be of a server which restarted, so those before it was watched
	// aren't handled
	s := newServer(t)
	hook := s.webhook(t, "")
	s.call(t, hook, "1")
	s.call(t, hook, "2")
	out := s.run(t)
	eventually(t, func() bool { return out.count("webhook calls are watched") == 1 })
	s.fail.Store(1)
	s.call(t, hook, "3")
	eventually(t, func() bool { return out.count("call=3 ") == 1 })
	assert.Equal(t, 1, out.count(`msg="webhook call" `))
}
