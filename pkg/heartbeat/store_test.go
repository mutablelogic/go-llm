package heartbeat_test

import (
	"context"
	"os"
	"testing"
	"time"

	heartbeat "github.com/mutablelogic/go-llm/pkg/heartbeat"
	file "github.com/mutablelogic/go-llm/pkg/heartbeat/file"
	assert "github.com/stretchr/testify/assert"
)

func newTestStore(t *testing.T) *file.Store {
	t.Helper()
	dir, err := os.MkdirTemp("", "heartbeat-store-*")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	s, err := file.NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func futureSpec(t *testing.T) heartbeat.TimeSpec {
	t.Helper()
	ts, err := heartbeat.NewTimeSpec("* * * * *", nil)
	if err != nil {
		t.Fatal(err)
	}
	return ts
}

// nextMinuteSpec returns a one-shot TimeSpec for 1 minute from now (via NewTimeSpec,
// so it passes future-time validation). Pair with a store whose NowFn advances
// the clock by >=2 minutes so Next() treats it as due.
func nextMinuteSpec(t *testing.T) heartbeat.TimeSpec {
	t.Helper()
	at := time.Now().UTC().Truncate(time.Minute).Add(time.Minute)
	ts, err := heartbeat.NewTimeSpec(at, nil)
	if err != nil {
		t.Fatal(err)
	}
	return ts
}

// advancedStore returns a *file.Store whose internal clock is 2 minutes ahead
// of real time, so heartbeats created with nextMinuteSpec / futureSpec are
// immediately treated as due by Next().
func advancedStore(t *testing.T) *file.Store {
	t.Helper()
	s := newTestStore(t)
	s.NowFn = func() time.Time { return time.Now().Add(2 * time.Minute) }
	return s
}

func Test_store_001(t *testing.T) {
	assert := assert.New(t)
	_, err := file.NewStore("")
	assert.Error(err)
}

func Test_store_002(t *testing.T) {
	assert := assert.New(t)
	s := newTestStore(t)
	assert.NotNil(s)
}

func Test_store_003(t *testing.T) {
	assert := assert.New(t)
	s := newTestStore(t)
	_, err := s.Create(context.Background(), heartbeat.HeartbeatMeta{Message: "", Schedule: futureSpec(t)})
	assert.Error(err)
}

func Test_store_004(t *testing.T) {
	assert := assert.New(t)
	s := newTestStore(t)
	h, err := s.Create(context.Background(), heartbeat.HeartbeatMeta{Message: "test message", Schedule: futureSpec(t)})
	assert.NoError(err)
	assert.NotNil(h)
	assert.NotEmpty(h.ID)
	assert.Equal("test message", h.Message)
	assert.False(h.Created.IsZero())
	assert.Nil(h.Modified)
}

func Test_store_005(t *testing.T) {
	assert := assert.New(t)
	s := newTestStore(t)
	_, err := s.Get(context.Background(), "does-not-exist")
	assert.Error(err)
}

func Test_store_006(t *testing.T) {
	assert := assert.New(t)
	s := newTestStore(t)
	created, err := s.Create(context.Background(), heartbeat.HeartbeatMeta{Message: "hello", Schedule: futureSpec(t)})
	assert.NoError(err)
	got, err := s.Get(context.Background(), created.ID)
	assert.NoError(err)
	assert.Equal(created.ID, got.ID)
	assert.Equal(created.Message, got.Message)
}

func Test_store_007(t *testing.T) {
	assert := assert.New(t)
	s := newTestStore(t)
	err := s.Delete(context.Background(), "does-not-exist")
	assert.Error(err)
}

func Test_store_008(t *testing.T) {
	assert := assert.New(t)
	s := newTestStore(t)
	h, err := s.Create(context.Background(), heartbeat.HeartbeatMeta{Message: "bye", Schedule: futureSpec(t)})
	assert.NoError(err)
	assert.NoError(s.Delete(context.Background(), h.ID))
	_, err = s.Get(context.Background(), h.ID)
	assert.Error(err)
}

func Test_store_009(t *testing.T) {
	assert := assert.New(t)
	s := newTestStore(t)
	list, err := s.List(context.Background(), false)
	assert.NoError(err)
	assert.Empty(list)
}

func Test_store_010(t *testing.T) {
	assert := assert.New(t)
	s := newTestStore(t)
	_, err := s.Create(context.Background(), heartbeat.HeartbeatMeta{Message: "a", Schedule: futureSpec(t)})
	assert.NoError(err)
	_, err = s.Create(context.Background(), heartbeat.HeartbeatMeta{Message: "b", Schedule: futureSpec(t)})
	assert.NoError(err)
	all, err := s.List(context.Background(), true)
	assert.NoError(err)
	assert.Len(all, 2)
}

func Test_store_011(t *testing.T) {
	assert := assert.New(t)
	s := newTestStore(t)
	h, err := s.Create(context.Background(), heartbeat.HeartbeatMeta{Message: "original", Schedule: futureSpec(t)})
	assert.NoError(err)
	updated, err := s.Update(context.Background(), h.ID, heartbeat.HeartbeatMeta{Message: "updated message"})
	assert.NoError(err)
	assert.Equal("updated message", updated.Message)
}

func Test_store_012(t *testing.T) {
	assert := assert.New(t)
	s := advancedStore(t)
	h, err := s.Create(context.Background(), heartbeat.HeartbeatMeta{Message: "one-shot", Schedule: nextMinuteSpec(t)})
	assert.NoError(err)
	fired, err := s.Next(context.Background())
	assert.NoError(err)
	if assert.Len(fired, 1) {
		assert.Equal(h.ID, fired[0].ID)
		assert.True(fired[0].Fired)
	}
	// Rescheduling a fired one-shot should reset fired=false.
	newTs, err := heartbeat.NewTimeSpec("0 9 * * 1-5", nil)
	assert.NoError(err)
	updated, err := s.Update(context.Background(), h.ID, heartbeat.HeartbeatMeta{Schedule: newTs})
	assert.NoError(err)
	assert.False(updated.Fired)
}

func Test_store_013(t *testing.T) {
	assert := assert.New(t)
	s := newTestStore(t)
	_, err := s.Update(context.Background(), "no-such-id", heartbeat.HeartbeatMeta{Message: "msg"})
	assert.Error(err)
}

func Test_store_015(t *testing.T) {
	assert := assert.New(t)
	s := advancedStore(t)
	h, err := s.Create(context.Background(), heartbeat.HeartbeatMeta{Message: "one-shot", Schedule: nextMinuteSpec(t)})
	assert.NoError(err)
	_, err = s.Next(context.Background())
	assert.NoError(err)
	got, err := s.Get(context.Background(), h.ID)
	assert.NoError(err)
	assert.True(got.Fired)
	assert.Nil(got.LastFired)
}

func Test_store_016(t *testing.T) {
	assert := assert.New(t)
	s := advancedStore(t)
	h, err := s.Create(context.Background(), heartbeat.HeartbeatMeta{Message: "recurring", Schedule: futureSpec(t)})
	assert.NoError(err)
	_, err = s.Next(context.Background())
	assert.NoError(err)
	got, err := s.Get(context.Background(), h.ID)
	assert.NoError(err)
	assert.False(got.Fired)
	assert.NotNil(got.LastFired)
}

func Test_store_017(t *testing.T) {
	assert := assert.New(t)
	s := newTestStore(t)
	fired, err := s.Next(context.Background())
	assert.NoError(err)
	assert.Empty(fired)
}

func Test_store_018(t *testing.T) {
	assert := assert.New(t)
	s := advancedStore(t)
	h, err := s.Create(context.Background(), heartbeat.HeartbeatMeta{Message: "fired", Schedule: nextMinuteSpec(t)})
	assert.NoError(err)
	// First Next() should collect and fire it.
	fired, err := s.Next(context.Background())
	assert.NoError(err)
	if assert.Len(fired, 1) {
		assert.Equal(h.ID, fired[0].ID)
	}
	// Second Next() must not return it again.
	again, err := s.Next(context.Background())
	assert.NoError(err)
	for _, d := range again {
		assert.NotEqual(h.ID, d.ID)
	}
}
