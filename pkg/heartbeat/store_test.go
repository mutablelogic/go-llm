package heartbeat_test

import (
	"os"
	"testing"
	"time"

	heartbeat "github.com/mutablelogic/go-llm/pkg/heartbeat"
	file "github.com/mutablelogic/go-llm/pkg/heartbeat/file"
	assert "github.com/stretchr/testify/assert"
)

func newTestStore(t *testing.T) heartbeat.Store {
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
	_, err := s.Create("", futureSpec(t))
	assert.Error(err)
}

func Test_store_004(t *testing.T) {
	assert := assert.New(t)
	s := newTestStore(t)
	h, err := s.Create("test message", futureSpec(t))
	assert.NoError(err)
	assert.NotNil(h)
	assert.NotEmpty(h.ID)
	assert.Equal("test message", h.Message)
	assert.False(h.Created.IsZero())
	assert.False(h.Modified.IsZero())
}

func Test_store_005(t *testing.T) {
	assert := assert.New(t)
	s := newTestStore(t)
	_, err := s.Get("does-not-exist")
	assert.Error(err)
}

func Test_store_006(t *testing.T) {
	assert := assert.New(t)
	s := newTestStore(t)
	created, err := s.Create("hello", futureSpec(t))
	assert.NoError(err)
	got, err := s.Get(created.ID)
	assert.NoError(err)
	assert.Equal(created.ID, got.ID)
	assert.Equal(created.Message, got.Message)
}

func Test_store_007(t *testing.T) {
	assert := assert.New(t)
	s := newTestStore(t)
	err := s.Delete("does-not-exist")
	assert.Error(err)
}

func Test_store_008(t *testing.T) {
	assert := assert.New(t)
	s := newTestStore(t)
	h, err := s.Create("bye", futureSpec(t))
	assert.NoError(err)
	assert.NoError(s.Delete(h.ID))
	_, err = s.Get(h.ID)
	assert.Error(err)
}

func Test_store_009(t *testing.T) {
	assert := assert.New(t)
	s := newTestStore(t)
	list, err := s.List(false)
	assert.NoError(err)
	assert.Empty(list)
}

func Test_store_010(t *testing.T) {
	assert := assert.New(t)
	s := newTestStore(t)
	_, err := s.Create("a", futureSpec(t))
	assert.NoError(err)
	_, err = s.Create("b", futureSpec(t))
	assert.NoError(err)
	all, err := s.List(true)
	assert.NoError(err)
	assert.Len(all, 2)
}

func Test_store_011(t *testing.T) {
	assert := assert.New(t)
	s := newTestStore(t)
	h, err := s.Create("original", futureSpec(t))
	assert.NoError(err)
	updated, err := s.Update(h.ID, "updated message", nil)
	assert.NoError(err)
	assert.Equal("updated message", updated.Message)
}

func Test_store_012(t *testing.T) {
	assert := assert.New(t)
	s := newTestStore(t)
	ts, err := heartbeat.NewTimeSpec(time.Now().UTC().Add(2*time.Hour), nil)
	assert.NoError(err)
	h, err := s.Create("one-shot", ts)
	assert.NoError(err)
	assert.NoError(s.MarkFired(h.ID))
	got, err := s.Get(h.ID)
	assert.NoError(err)
	assert.True(got.Fired)
	newTs := futureSpec(t)
	updated, err := s.Update(h.ID, "", &newTs)
	assert.NoError(err)
	assert.False(updated.Fired)
}

func Test_store_013(t *testing.T) {
	assert := assert.New(t)
	s := newTestStore(t)
	_, err := s.Update("no-such-id", "msg", nil)
	assert.Error(err)
}

func Test_store_014(t *testing.T) {
	assert := assert.New(t)
	s := newTestStore(t)
	err := s.MarkFired("no-such-id")
	assert.Error(err)
}

func Test_store_015(t *testing.T) {
	assert := assert.New(t)
	s := newTestStore(t)
	ts, err := heartbeat.NewTimeSpec(time.Now().UTC().Add(time.Hour), nil)
	assert.NoError(err)
	h, err := s.Create("one-shot", ts)
	assert.NoError(err)
	assert.NoError(s.MarkFired(h.ID))
	got, err := s.Get(h.ID)
	assert.NoError(err)
	assert.True(got.Fired)
	assert.Nil(got.LastFired)
}

func Test_store_016(t *testing.T) {
	assert := assert.New(t)
	s := newTestStore(t)
	h, err := s.Create("recurring", futureSpec(t))
	assert.NoError(err)
	assert.NoError(s.MarkFired(h.ID))
	got, err := s.Get(h.ID)
	assert.NoError(err)
	assert.False(got.Fired)
	assert.NotNil(got.LastFired)
}

func Test_store_017(t *testing.T) {
	assert := assert.New(t)
	s := newTestStore(t)
	due, err := s.Due()
	assert.NoError(err)
	assert.Empty(due)
}

func Test_store_018(t *testing.T) {
	assert := assert.New(t)
	s := newTestStore(t)
	ts, err := heartbeat.NewTimeSpec(time.Now().UTC().Add(time.Hour), nil)
	assert.NoError(err)
	h, err := s.Create("fired", ts)
	assert.NoError(err)
	assert.NoError(s.MarkFired(h.ID))
	due, err := s.Due()
	assert.NoError(err)
	for _, d := range due {
		assert.NotEqual(h.ID, d.ID)
	}
}
