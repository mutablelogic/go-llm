package webhook

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
	"uuid"

	// Packages
	schema "github.com/mutablelogic/go-llm/webhook/schema"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

///////////////////////////////////////////////////////////////////////////////
// HELPERS

// handled records the calls which are handled, and fails those of the
// sessions whose errors are set
type handled struct {
	mu    sync.Mutex
	calls []Calls
	errs  map[uuid.UUID]error
}

func (h *handled) handle(_ context.Context, calls *Calls) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.calls = append(h.calls, *calls)
	return h.errs[calls.Session]
}

// of returns the calls which were handled of the session, each time
func (h *handled) of(session uuid.UUID) [][]uint64 {
	h.mu.Lock()
	defer h.mu.Unlock()
	var result [][]uint64
	for _, calls := range h.calls {
		if calls.Session == session {
			result = append(result, calls.ids())
		}
	}
	return result
}

func (h *handled) fail(session uuid.UUID, err error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.errs == nil {
		h.errs = make(map[uuid.UUID]error)
	}
	h.errs[session] = err
}

///////////////////////////////////////////////////////////////////////////////
// TESTS

func Test_Run(t *testing.T) {
	// The calls received before it runs are handled, those of each session
	// together, once each, and aren't pending then, apart from those which
	// were refused, which aren't handled
	s := newServer(t)
	a, signed := s.webhook(t, ""), s.webhook(t, "s3cret")
	b, err := s.registry.CreateWebhook(t.Context(), schema.WebhookMeta{Session: a.Session, Description: "Another"})
	require.NoError(t, err)
	other := s.webhook(t, "")
	s.call(t, a, "what was sent")
	s.call(t, signed, "not signed")
	s.call(t, other, "other")
	s.call(t, b, "3")
	var h handled
	out := s.run(t, h.handle)
	eventually(t, func() bool { return len(s.pending(t)) == 0 })
	assert.Equal(t, [][]uint64{{1, 4}}, h.of(a.Session))
	assert.Equal(t, [][]uint64{{3}}, h.of(other.Session))
	assert.Empty(t, h.of(signed.Session))
	for _, calls := range h.calls {
		if calls.Session == a.Session {
			require.Len(t, calls.Webhooks, 2)
			assert.Equal(t, []uint64{a.ID, b.ID}, []uint64{calls.Webhooks[0].ID, calls.Webhooks[1].ID})
		}
	}

	// A burst is handled together
	for range 3 {
		s.call(t, a, "burst")
	}
	eventually(t, func() bool { return len(s.pending(t)) == 0 })
	assert.Equal(t, [][]uint64{{1, 4}, {5, 6, 7}}, h.of(a.Session))

	// It's logged, without what was sent
	logged := out.String()
	assert.Contains(t, logged, "webhook calls handled")
	for _, text := range []string{"what was sent", "not signed", "s3cret", a.Token, signed.Token} {
		assert.NotContains(t, logged, text)
	}
}

func Test_RunMany(t *testing.T) {
	// More calls than are handled at once are handled, in order, once each
	s := newServer(t)
	w := s.webhook(t, "")
	want := []uint64{}
	for id := range uint64(2*handleLimit + 3) {
		s.call(t, w, "x")
		want = append(want, id+1)
	}
	var h handled
	s.run(t, h.handle)
	eventually(t, func() bool { return len(s.pending(t)) == 0 })
	var got []uint64
	for _, ids := range h.of(w.Session) {
		assert.LessOrEqual(t, len(ids), handleLimit)
		got = append(got, ids...)
	}
	assert.Equal(t, want, got)
}

func Test_RunFails(t *testing.T) {
	// Calls which can't be handled are tried again, a few times, and don't
	// stop another session's
	s := newServer(t)
	bad, good := s.webhook(t, ""), s.webhook(t, "")
	var h handled
	h.fail(bad.Session, errors.New("no model"))
	out := s.run(t, h.handle)
	s.call(t, bad, "1")
	s.call(t, good, "2")
	eventually(t, func() bool { return len(h.of(good.Session)) == 1 })
	eventually(t, func() bool { return len(s.pending(t)) == 0 })
	assert.Len(t, h.of(bad.Session), handleTries)
	assert.Len(t, h.of(good.Session), 1)
	assert.Equal(t, handleTries-1, out.count("webhook calls not handled\""))
	assert.Equal(t, 1, out.count("not tried again"))

	// Which are handled, when one of them works
	h.fail(bad.Session, errors.New("no model"))
	s.call(t, bad, "3")
	eventually(t, func() bool { return len(h.of(bad.Session)) == handleTries+1 })
	h.fail(bad.Session, nil)
	eventually(t, func() bool { return len(s.pending(t)) == 0 })
	assert.Len(t, h.of(bad.Session), handleTries+2)
}

func Test_RunNotMarked(t *testing.T) {
	// Calls which were handled, and couldn't be marked as seen, are marked
	// later, and aren't handled again
	s := newServer(t)
	w := s.webhook(t, "")
	s.call(t, w, "x")
	s.fail.Store(2)
	var h handled
	s.run(t, h.handle)
	eventually(t, func() bool { return len(s.pending(t)) == 0 })
	assert.Equal(t, [][]uint64{{1}}, h.of(w.Session))
}

func Test_RunReconnects(t *testing.T) {
	// The calls which are received while it can't reach the server are
	// handled once it can
	s := newServer(t)
	s.down.Store(true)
	w := s.webhook(t, "")
	var h handled
	out := s.run(t, h.handle)
	eventually(t, func() bool { return out.count("webhook calls aren't watched") > 1 })
	s.call(t, w, "1")
	s.down.Store(false)
	eventually(t, func() bool { return len(s.pending(t)) == 0 })
	s.down.Store(true)
	s.CloseClientConnections()
	watched := out.count("webhook calls are watched")
	s.call(t, w, "2")
	s.down.Store(false)
	eventually(t, func() bool { return out.count("webhook calls are watched") > watched })
	eventually(t, func() bool { return len(s.pending(t)) == 0 })
	assert.Equal(t, [][]uint64{{1}, {2}}, h.of(w.Session))
}

func Test_RunWebhookDeleted(t *testing.T) {
	// A session whose webhooks are deleted, as its calls are tried again,
	// isn't tried again
	s := newServer(t)
	w := s.webhook(t, "")
	s.call(t, w, "x")
	tried := make(chan struct{}, handleTries)
	h := newHandler(s.client(t), func(context.Context, *Calls) error {
		tried <- struct{}{}
		return errors.New("no model")
	}, discard)
	require.NoError(t, h.start(t.Context()))
	<-tried
	_, err := s.registry.DeleteWebhook(t.Context(), w.ID)
	require.NoError(t, err)
	h.wg.Wait()
	assert.Empty(t, tried)
	assert.Empty(t, h.active)
}

func Test_RunTriesOfEachCall(t *testing.T) {
	// A call which is received as others are tried again has tries of its
	// own, so it isn't marked as seen with them
	s := newServer(t)
	w := s.webhook(t, "")
	s.call(t, w, "1")
	var h handled
	h.fail(w.Session, errors.New("no model"))
	count := 0
	handler := newHandler(s.client(t), func(ctx context.Context, calls *Calls) error {
		if count++; count == handleTries-1 {
			s.call(t, w, "2")
		}
		return h.handle(ctx, calls)
	}, discard)
	require.NoError(t, handler.start(t.Context()))
	handler.wg.Wait()
	tried := make(map[uint64]int)
	for _, ids := range h.of(w.Session) {
		for _, id := range ids {
			tried[id]++
		}
	}
	assert.Equal(t, map[uint64]int{1: handleTries, 2: handleTries}, tried)
	assert.Empty(t, s.pending(t))
}

func Test_RunSessionsDontWait(t *testing.T) {
	// A session, even one with more calls than are handled at once, before
	// another's, doesn't keep it waiting, and one which is received as its
	// session's calls are handled is handled after them
	s := newServer(t)
	slow, fast := s.webhook(t, ""), s.webhook(t, "")
	for range handleLimit {
		s.call(t, slow, "slow")
	}
	s.call(t, fast, "fast")
	var h handled
	release := make(chan struct{})
	s.run(t, func(ctx context.Context, calls *Calls) error {
		if calls.Session == slow.Session {
			select {
			case <-release:
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		return h.handle(ctx, calls)
	})
	eventually(t, func() bool { return len(s.pending(t)) == handleLimit })
	assert.Equal(t, [][]uint64{{handleLimit + 1}}, h.of(fast.Session))
	s.call(t, slow, "more")
	s.call(t, fast, "again")
	eventually(t, func() bool { return len(h.of(fast.Session)) == 2 })
	assert.Empty(t, h.of(slow.Session))
	close(release)
	eventually(t, func() bool { return len(s.pending(t)) == 0 })
	require.Len(t, h.of(slow.Session), 2)
	assert.Len(t, h.of(slow.Session)[0], handleLimit)
	assert.Equal(t, []uint64{handleLimit + 2}, h.of(slow.Session)[1])
}

func Test_Input(t *testing.T) {
	received := time.Date(2026, 11, 25, 10, 0, 0, 0, time.FixedZone("CET", 3600))
	calls := Calls{
		Session:  uuid.New(),
		Webhooks: []schema.Webhook{{ID: 4, WebhookMeta: schema.WebhookMeta{Type: "github", Description: "Issues. Say the title", Secret: "s3cret"}}},
		Calls: []schema.Call{
			{ID: 12, Webhook: 4, Received: received, Event: "issues.opened", Verified: true, Values: json.RawMessage(`{"action":"opened","url":"https://example.com/api/hook/` + strings.Repeat("a", 43) + `"}`)},
			{ID: 13, Webhook: 4, Received: received, Test: true, Body: []byte{1, 2, 3}, Headers: map[string]string{"X-Hub-Signature-256": "signed"}},
		},
	}
	data, err := calls.Input(t.Context(), []string{"Now: 10:00", "Person's name: Sam"})
	require.NoError(t, err)
	assert.JSONEq(t, `{
		"webhooks": [{"id":4,"type":"github","description":"Issues. Say the title"}],
		"calls": [
			{"id":12,"webhook":4,"received":"2026-11-25T09:00:00Z","event":"issues.opened","signature":"verified","values":{"action":"opened","url":"https://example.com/api/hook/{token}"}},
			{"id":13,"webhook":4,"received":"2026-11-25T09:00:00Z","signature":"not_checked","test":true,"bytes":3}
		],
		"context": "Now: 10:00\nPerson's name: Sam"
	}`, string(data))

	// What was sent, and its event, are shortened, so they fit, however many
	// calls there are, and however much of each was sent
	long := strings.Repeat("long ", 20000)
	for _, n := range []int{1, handleLimit} {
		calls.Calls = nil
		for range n {
			calls.Calls = append(calls.Calls, schema.Call{ID: 1, Webhook: 4, Event: strings.Repeat("é", 100000), Values: json.RawMessage(`{"text":"` + long + `"}`), Query: map[string][]string{"q": {long}, long: {"x"}}})
		}
		data, err := calls.Input(t.Context(), nil)
		require.NoError(t, err)
		assert.Less(t, len(data), sentTokens*tokenBytes+n*(2*eventLimit+200)+500)
		var input agentInput
		require.NoError(t, json.Unmarshal(data, &input))
		assert.Len(t, []rune(input.Calls[0].Event), eventLimit)
		assert.Contains(t, string(input.Calls[0].Values), "long long")
		assert.NotEmpty(t, input.Calls[0].Query)
	}

	// A short query leaves its tokens for the values
	calls.Calls = []schema.Call{{ID: 1, Webhook: 4, Query: map[string][]string{"q": {"short"}}, Values: json.RawMessage(`{"text":"` + long + `"}`)}}
	data, err = calls.Input(t.Context(), nil)
	require.NoError(t, err)
	assert.Contains(t, string(data), `"query":{"q":["short"]}`)
	assert.Greater(t, len(data), sentTokens*tokenBytes*8/10)
}
