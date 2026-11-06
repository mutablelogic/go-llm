package heartbeat_test

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"
	"uuid"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	broadcaster "github.com/mutablelogic/go-llm/broadcaster"
	eventschema "github.com/mutablelogic/go-llm/broadcaster/schema"
	heartbeat "github.com/mutablelogic/go-llm/heartbeat"
	schema "github.com/mutablelogic/go-llm/heartbeat/schema"
	registry "github.com/mutablelogic/go-llm/registry"
	types "github.com/mutablelogic/go-server/pkg/types"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

///////////////////////////////////////////////////////////////////////////////
// HELPERS

// clock is a clock which is set, and moved on, by the test
type clock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *clock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *clock) Set(s string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	c.now = t
}

// newHeartbeats returns heartbeats in the store, with the clock, and a
// broadcaster, which is running, and a channel of its events
func newHeartbeats(t *testing.T, store heartbeat.Store, c *clock, opts ...heartbeat.Opt) (*heartbeat.Heartbeats, *broadcaster.Broadcaster, chan eventschema.Event) {
	t.Helper()
	b := broadcaster.New()
	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)
	go b.Run(ctx)
	ch := make(chan eventschema.Event, 64)
	_, err := b.Watch(ctx, ch)
	require.NoError(t, err)
	h, err := heartbeat.New(store, append([]heartbeat.Opt{heartbeat.WithBroadcaster(b)}, opts...)...)
	require.NoError(t, err)
	heartbeat.SetClock(h, c.Now)
	return h, b, ch
}

// fires returns the fires of the events which were published
func fires(t *testing.T, ch chan eventschema.Event) []schema.FireEvent {
	t.Helper()
	var result []schema.FireEvent
	for {
		select {
		case event := <-ch:
			if event.Schema == heartbeat.Name && event.Table == heartbeat.FireTable {
				var fire schema.FireEvent
				require.NoError(t, json.Unmarshal(event.Data, &fire))
				result = append(result, fire)
			}
		default:
			return result
		}
	}
}

func at(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return t
}

///////////////////////////////////////////////////////////////////////////////
// TESTS

func Test_Create(t *testing.T) {
	c := &clock{}
	c.Set("2026-01-05T08:00:00Z")
	h, _, _ := newHeartbeats(t, heartbeat.NewStore(), c)
	session := uuid.New()

	for name, meta := range map[string]schema.HeartbeatMeta{
		"No title":         {Prompt: "x", Session: session, Schedule: "0 9 * * *"},
		"Two lines":        {Title: "a\nb", Prompt: "x", Session: session, Schedule: "0 9 * * *"},
		"No prompt":        {Title: "x", Session: session, Schedule: "0 9 * * *"},
		"No session":       {Title: "x", Prompt: "x", Schedule: "0 9 * * *"},
		"Neither":          {Title: "x", Prompt: "x", Session: session},
		"Both":             {Title: "x", Prompt: "x", Session: session, Schedule: "0 9 * * *", At: types.Ptr(at("2026-01-06T09:00:00Z"))},
		"Invalid schedule": {Title: "x", Prompt: "x", Session: session, Schedule: "0 25 * * *"},
		"Unknown zone":     {Title: "x", Prompt: "x", Session: session, Schedule: "0 9 * * *", Timezone: "Mars/Olympus"},
		"Local zone":       {Title: "x", Prompt: "x", Session: session, Schedule: "0 9 * * *", Timezone: "Local"},
		"Zone with a time": {Title: "x", Prompt: "x", Session: session, At: types.Ptr(at("2026-01-06T09:00:00Z")), Timezone: "Europe/Berlin"},
		"Time has passed":  {Title: "x", Prompt: "x", Session: session, At: types.Ptr(at("2026-01-05T07:00:00Z"))},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := h.Create(t.Context(), meta)
			assert.ErrorIs(t, err, llm.ErrBadParameter)
		})
	}

	// A schedule, which is made canonical, and fires next at nine
	got, err := h.Create(t.Context(), schema.HeartbeatMeta{Title: " Briefing ", Prompt: "Brief me", Session: session, Schedule: "0 9 * * 1-5", Timezone: "Europe/Berlin"})
	require.NoError(t, err)
	assert.Equal(t, "Briefing", got.Title)
	require.NotNil(t, got.Next)
	assert.Equal(t, at("2026-01-05T08:00:00Z"), *got.Next)

	// A time, with its offset, which is in UTC
	got, err = h.Create(t.Context(), schema.HeartbeatMeta{Title: "Call", Prompt: "Remind them to call", Session: session, At: types.Ptr(at("2026-01-05T15:00:00+02:00"))})
	require.NoError(t, err)
	assert.Equal(t, at("2026-01-05T13:00:00Z"), *got.At)
	assert.Equal(t, at("2026-01-05T13:00:00Z"), *got.Next)

	// Listed, the soonest first
	list, err := h.List(t.Context(), schema.HeartbeatListRequest{})
	require.NoError(t, err)
	require.Equal(t, uint(2), list.Count)
	assert.Equal(t, "Briefing", list.Body[0].Title)
	list, err = h.List(t.Context(), schema.HeartbeatListRequest{Session: uuid.New().String()})
	require.NoError(t, err)
	assert.Equal(t, uint(0), list.Count)
	_, err = h.List(t.Context(), schema.HeartbeatListRequest{Session: "x"})
	assert.ErrorIs(t, err, llm.ErrBadParameter)
}

func Test_FireOnce(t *testing.T) {
	c := &clock{}
	c.Set("2026-01-05T08:00:00Z")
	h, _, ch := newHeartbeats(t, heartbeat.NewStore(), c)
	session := uuid.New()
	once, err := h.Create(t.Context(), schema.HeartbeatMeta{Title: "Call", Prompt: "Remind them to call", Session: session, At: types.Ptr(at("2026-01-05T09:00:00Z"))})
	require.NoError(t, err)

	// Not before its time
	c.Set("2026-01-05T08:59:00Z")
	heartbeat.Tick(t.Context(), h)
	assert.Empty(t, fires(t, ch))

	// At its time, and then not again
	c.Set("2026-01-05T09:00:00.1Z")
	heartbeat.Tick(t.Context(), h)
	got := fires(t, ch)
	require.Len(t, got, 1)
	assert.Equal(t, schema.FireEvent{Heartbeat: once.ID, Title: "Call", Prompt: "Remind them to call", Session: session, Scheduled: at("2026-01-05T09:00:00Z"), Fired: at("2026-01-05T09:00:00.1Z")}, got[0])
	c.Set("2026-01-05T09:01:00Z")
	heartbeat.Tick(t.Context(), h)
	assert.Empty(t, fires(t, ch))
	once, err = h.Get(t.Context(), once.ID)
	require.NoError(t, err)
	assert.Nil(t, once.Next)
	assert.NotNil(t, once.Fired)

	// A new time fires again
	once, err = h.Update(t.Context(), once.ID, schema.HeartbeatRequest{At: types.Ptr(at("2026-01-05T10:00:00Z"))})
	require.NoError(t, err)
	assert.Nil(t, once.Fired)
	c.Set("2026-01-05T10:00:00Z")
	heartbeat.Tick(t.Context(), h)
	assert.Len(t, fires(t, ch), 1)
}

func Test_FireSchedule(t *testing.T) {
	c := &clock{}
	c.Set("2026-01-05T08:00:00Z")
	h, _, ch := newHeartbeats(t, heartbeat.NewStore(), c)
	_, err := h.Create(t.Context(), schema.HeartbeatMeta{Title: "Briefing", Prompt: "Brief me", Session: uuid.New(), Schedule: "0 9 * * *"})
	require.NoError(t, err)

	// At nine, once in the minute
	c.Set("2026-01-05T09:00:00Z")
	heartbeat.Tick(t.Context(), h)
	c.Set("2026-01-05T09:00:30Z")
	heartbeat.Tick(t.Context(), h)
	assert.Len(t, fires(t, ch), 1)

	// Three days later, as the server wasn't running, once, when it was
	// first due
	c.Set("2026-01-08T12:00:00Z")
	heartbeat.Tick(t.Context(), h)
	got := fires(t, ch)
	require.Len(t, got, 1)
	assert.Equal(t, at("2026-01-06T09:00:00Z"), got[0].Scheduled)
	assert.Equal(t, at("2026-01-08T12:00:00Z"), got[0].Fired)
	c.Set("2026-01-08T12:01:00Z")
	heartbeat.Tick(t.Context(), h)
	assert.Empty(t, fires(t, ch))

	// And the next day
	list, err := h.List(t.Context(), schema.HeartbeatListRequest{})
	require.NoError(t, err)
	assert.Equal(t, at("2026-01-09T09:00:00Z"), *list.Body[0].Next)
}

func Test_FireShared(t *testing.T) {
	// Two servers, which share a store, fire a heartbeat once
	c := &clock{}
	c.Set("2026-01-05T08:00:00Z")
	store := heartbeat.NewStore()
	a, b1, chA := newHeartbeats(t, store, c)
	b, _, chB := newHeartbeats(t, store, c)
	_ = b1
	for range 10 {
		_, err := a.Create(t.Context(), schema.HeartbeatMeta{Title: "Briefing", Prompt: "Brief me", Session: uuid.New(), Schedule: "*/5 * * * *"})
		require.NoError(t, err)
	}
	c.Set("2026-01-05T08:05:00Z")
	var wg sync.WaitGroup
	for _, h := range []*heartbeat.Heartbeats{a, b} {
		wg.Go(func() {
			heartbeat.Tick(t.Context(), h)
		})
	}
	wg.Wait()
	assert.Len(t, append(fires(t, chA), fires(t, chB)...), 10)
}

func Test_FollowZone(t *testing.T) {
	berlin, err := time.LoadLocation("Europe/Berlin")
	require.NoError(t, err)
	tokyo, err := time.LoadLocation("Asia/Tokyo")
	require.NoError(t, err)
	var mu sync.Mutex
	zone := berlin
	c := &clock{}
	c.Set("2026-01-04T06:00:00Z")
	h, _, ch := newHeartbeats(t, heartbeat.NewStore(), c, heartbeat.WithZone(func() *time.Location {
		mu.Lock()
		defer mu.Unlock()
		return zone
	}))
	follows, err := h.Create(t.Context(), schema.HeartbeatMeta{Title: "Briefing", Prompt: "Brief me", Session: uuid.New(), Schedule: "0 9 * * *"})
	require.NoError(t, err)
	fixed, err := h.Create(t.Context(), schema.HeartbeatMeta{Title: "Fixed", Prompt: "Brief me", Session: uuid.New(), Schedule: "0 9 * * *", Timezone: "Europe/Berlin"})
	require.NoError(t, err)
	assert.Equal(t, at("2026-01-04T08:00:00Z"), *follows.Next)
	assert.Equal(t, at("2026-01-04T08:00:00Z"), *fixed.Next)

	// Both fire at nine in Berlin, and not before the next day
	c.Set("2026-01-04T08:00:00Z")
	heartbeat.Tick(t.Context(), h)
	assert.Len(t, fires(t, ch), 2)
	c.Set("2026-01-05T06:00:00Z")
	heartbeat.Tick(t.Context(), h)
	assert.Empty(t, fires(t, ch))

	// The person goes to Tokyo, where nine today has passed, so the
	// heartbeat which follows them doesn't fire for it, but tomorrow, at
	// nine there, and the other at nine in Berlin
	mu.Lock()
	zone = tokyo
	mu.Unlock()
	heartbeat.Tick(t.Context(), h)
	assert.Empty(t, fires(t, ch))
	follows, err = h.Get(t.Context(), follows.ID)
	require.NoError(t, err)
	assert.Equal(t, at("2026-01-06T00:00:00Z"), *follows.Next)
	fixed, err = h.Get(t.Context(), fixed.ID)
	require.NoError(t, err)
	assert.Equal(t, at("2026-01-05T08:00:00Z"), *fixed.Next)
}

func Test_DeleteSession(t *testing.T) {
	c := &clock{}
	c.Set("2026-01-05T08:00:00Z")
	h, b, _ := newHeartbeats(t, heartbeat.NewStore(), c)
	session, other := uuid.New(), uuid.New()
	for _, s := range []uuid.UUID{session, session, other} {
		_, err := h.Create(t.Context(), schema.HeartbeatMeta{Title: "Briefing", Prompt: "Brief me", Session: s, Schedule: "0 9 * * *"})
		require.NoError(t, err)
	}

	// Run, and then the session is deleted
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error)
	go func() { done <- h.Run(ctx, nil) }()
	require.Eventually(t, func() bool {
		b.Publish(eventschema.Event{Action: eventschema.EventDelete, Schema: registry.EventSchema, Table: registry.SessionTable, Row: session.String()})
		list, err := h.List(t.Context(), schema.HeartbeatListRequest{})
		return err == nil && list.Count == 1
	}, time.Second, 10*time.Millisecond)
	list, err := h.List(t.Context(), schema.HeartbeatListRequest{})
	require.NoError(t, err)
	assert.Equal(t, other, list.Body[0].Session)
	cancel()
	assert.NoError(t, <-done)
}

func Test_UpdatePassed(t *testing.T) {
	// A heartbeat whose time has passed can still have its title changed,
	// but not be given a time which has passed
	c := &clock{}
	c.Set("2026-01-05T08:00:00Z")
	h, _, _ := newHeartbeats(t, heartbeat.NewStore(), c)
	once, err := h.Create(t.Context(), schema.HeartbeatMeta{Title: "Call", Prompt: "Remind them to call", Session: uuid.New(), At: types.Ptr(at("2026-01-05T09:00:00Z"))})
	require.NoError(t, err)
	c.Set("2026-01-05T12:00:00Z")
	got, err := h.Update(t.Context(), once.ID, schema.HeartbeatRequest{Title: types.Ptr("Call Mum")})
	require.NoError(t, err)
	assert.Equal(t, "Call Mum", got.Title)
	_, err = h.Update(t.Context(), once.ID, schema.HeartbeatRequest{At: types.Ptr(at("2026-01-05T10:00:00Z"))})
	assert.ErrorIs(t, err, llm.ErrBadParameter)
}

func Test_Sessions(t *testing.T) {
	// With the sessions, a heartbeat is created for one which exists, and,
	// when its session is deleted, and the event is missed, it's deleted
	// when it's due, rather than fired
	var mu sync.Mutex
	sessions := map[uuid.UUID]bool{}
	exists := func(_ context.Context, id uuid.UUID) error {
		mu.Lock()
		defer mu.Unlock()
		if sessions[id] {
			return nil
		}
		return llm.ErrNotFound
	}
	c := &clock{}
	c.Set("2026-01-05T08:00:00Z")
	h, _, ch := newHeartbeats(t, heartbeat.NewStore(), c, heartbeat.WithSessions(exists))
	session := uuid.New()
	_, err := h.Create(t.Context(), schema.HeartbeatMeta{Title: "Briefing", Prompt: "Brief me", Session: session, Schedule: "0 9 * * *"})
	assert.ErrorIs(t, err, llm.ErrBadParameter)
	mu.Lock()
	sessions[session] = true
	mu.Unlock()
	_, err = h.Create(t.Context(), schema.HeartbeatMeta{Title: "Briefing", Prompt: "Brief me", Session: session, Schedule: "0 9 * * *"})
	require.NoError(t, err)

	mu.Lock()
	delete(sessions, session)
	mu.Unlock()
	c.Set("2026-01-05T09:00:00Z")
	heartbeat.Tick(t.Context(), h)
	assert.Empty(t, fires(t, ch))
	list, err := h.List(t.Context(), schema.HeartbeatListRequest{})
	require.NoError(t, err)
	assert.Equal(t, uint(0), list.Count)
}

func Test_Cleanup(t *testing.T) {
	// A heartbeat which won't fire again is kept for a week after it fired,
	// and then deleted, when it's next looked for, which is daily, and one
	// which fires again is kept
	c := &clock{}
	c.Set("2026-01-05T08:00:00Z")
	h, _, _ := newHeartbeats(t, heartbeat.NewStore(), c)
	once, err := h.Create(t.Context(), schema.HeartbeatMeta{Title: "Call", Prompt: "Remind them to call", Session: uuid.New(), At: types.Ptr(at("2026-01-05T09:00:00Z"))})
	require.NoError(t, err)
	daily, err := h.Create(t.Context(), schema.HeartbeatMeta{Title: "Briefing", Prompt: "Brief me", Session: uuid.New(), Schedule: "0 9 * * *"})
	require.NoError(t, err)
	count := func() uint {
		list, err := h.List(t.Context(), schema.HeartbeatListRequest{})
		require.NoError(t, err)
		return list.Count
	}

	// Fired, and kept, for a week
	c.Set("2026-01-05T09:00:00Z")
	heartbeat.Tick(t.Context(), h)
	assert.Equal(t, uint(2), count())
	c.Set("2026-01-12T08:59:00Z")
	heartbeat.Tick(t.Context(), h)
	assert.Equal(t, uint(2), count())

	// Not looked for again until a day after it was last looked for
	c.Set("2026-01-12T09:30:00Z")
	heartbeat.Tick(t.Context(), h)
	assert.Equal(t, uint(2), count())

	// And then deleted
	c.Set("2026-01-13T09:00:00Z")
	heartbeat.Tick(t.Context(), h)
	assert.Equal(t, uint(1), count())
	_, err = h.Get(t.Context(), once.ID)
	assert.ErrorIs(t, err, llm.ErrNotFound)
	_, err = h.Get(t.Context(), daily.ID)
	assert.NoError(t, err)
}

// changingStore is a store which reschedules a heartbeat once it's listed, as
// another server would, between cleanup's listing and its delete
type changingStore struct {
	heartbeat.Store
	id    uuid.UUID
	at    time.Time
	armed bool
}

func (s *changingStore) List(ctx context.Context) ([]schema.Heartbeat, error) {
	list, err := s.Store.List(ctx)
	if err == nil && s.armed {
		s.armed = false
		_, err = s.Store.Update(ctx, s.id, func(h *schema.Heartbeat) error {
			h.At, h.Fired, h.Modified = types.Ptr(s.at), nil, s.at
			return nil
		})
	}
	return list, err
}

func Test_CleanupChanged(t *testing.T) {
	// A heartbeat which was changed after cleanup listed it isn't deleted
	c := &clock{}
	c.Set("2026-01-05T08:00:00Z")
	store := &changingStore{Store: heartbeat.NewStore()}
	h, _, _ := newHeartbeats(t, store, c)
	once, err := h.Create(t.Context(), schema.HeartbeatMeta{Title: "Call", Prompt: "Remind them to call", Session: uuid.New(), At: types.Ptr(at("2026-01-05T09:00:00Z"))})
	require.NoError(t, err)
	c.Set("2026-01-05T09:00:00Z")
	heartbeat.Tick(t.Context(), h)

	// A week later, it's rescheduled as cleanup lists it
	store.id, store.at, store.armed = once.ID, at("2026-01-20T09:00:00Z"), true
	c.Set("2026-01-13T09:00:00Z")
	heartbeat.Tick(t.Context(), h)
	got, err := h.Get(t.Context(), once.ID)
	require.NoError(t, err)
	assert.Equal(t, at("2026-01-20T09:00:00Z"), *got.Next)
}
