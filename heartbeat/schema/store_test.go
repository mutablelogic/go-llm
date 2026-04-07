package pg_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	// Packages
	heartbeat "github.com/mutablelogic/go-llm/heartbeat/schema"
	schema "github.com/mutablelogic/go-llm/heartbeat/schema"
	test "github.com/mutablelogic/go-pg/pkg/test"
	assert "github.com/stretchr/testify/assert"
)

// Global connection variable
var conn test.Conn

// Start up a container and test the pool
func TestMain(m *testing.M) {
	test.Main(m, &conn)
}

func newTestStore(t *testing.T) *heartbeat.Store {
	t.Helper()
	c := conn.Begin(t)
	t.Cleanup(func() { c.Close() })
	s, err := heartbeat.NewStore(context.Background(), c)
	if err != nil {
		t.Fatal(err)
	}
	// Clean up any leftover data from previous tests
	if err := s.Exec(context.Background(), "TRUNCATE heartbeat.heartbeat"); err != nil {
		t.Fatal(err)
	}
	return s
}

func futureSpec(t *testing.T) schema.TimeSpec {
	t.Helper()
	ts, err := schema.NewTimeSpec("* * * * *", nil)
	if err != nil {
		t.Fatal(err)
	}
	return ts
}

// nextMinuteSpec returns a one-shot TimeSpec for 1 minute from now.
func nextMinuteSpec(t *testing.T) schema.TimeSpec {
	t.Helper()
	at := time.Now().UTC().Truncate(time.Minute).Add(time.Minute)
	ts, err := schema.NewTimeSpec(at, nil)
	if err != nil {
		t.Fatal(err)
	}
	return ts
}

// backdateHeartbeat updates a heartbeat's created timestamp and schedule in the database
// to make it appear due for Next().
// For this to work correctly with one-shot schedules:
// - created must be BEFORE the scheduled time (so Next finds the scheduled time)
// - scheduled time must be BEFORE now (so the heartbeat is due)
func backdateHeartbeat(t *testing.T, s *heartbeat.Store, id string) {
	t.Helper()
	now := time.Now().UTC()
	// Schedule for 1 minute ago
	scheduledTime := now.Add(-1 * time.Minute).Truncate(time.Minute)
	// Created 2 minutes ago (before the scheduled time)
	createdTime := now.Add(-2 * time.Minute)

	pastSchedule := fmt.Sprintf(`{"schedule":"%d %d %d %d * %d"}`,
		scheduledTime.Minute(), scheduledTime.Hour(), scheduledTime.Day(), int(scheduledTime.Month()), scheduledTime.Year())
	t.Logf("Backdating heartbeat %s: created=%s, schedule=%s", id, createdTime.Format(time.RFC3339), pastSchedule)

	// Update both created and schedule using the Store's connection (not With)
	// Using raw SQL that substitutes the values directly
	sql := fmt.Sprintf(`UPDATE heartbeat.heartbeat 
		SET created = '%s'::timestamptz,
		    schedule = '%s'::jsonb
		WHERE id = '%s'`, createdTime.Format(time.RFC3339), pastSchedule, id)
	if err := s.Exec(context.Background(), sql); err != nil {
		t.Fatalf("backdateHeartbeat exec failed: %v", err)
	}

	// Verify the update worked
	h, err := s.Get(context.Background(), id)
	if err != nil {
		t.Fatalf("backdateHeartbeat verify get failed: %v", err)
	}
	t.Logf("After backdate: schedule=%s (next=%s), created=%s", h.Schedule.String(), h.Schedule.Next(h.Created).Format(time.RFC3339), h.Created.UTC().Format(time.RFC3339))
}

// backdateCreatedOnly only updates the created timestamp without changing the schedule.
// This is useful for recurring schedules where we just need the heartbeat to be "due".
func backdateCreatedOnly(t *testing.T, s *heartbeat.Store, id string) {
	t.Helper()
	createdTime := time.Now().UTC().Add(-2 * time.Minute)
	t.Logf("Backdating created only for heartbeat %s: created=%s", id, createdTime.Format(time.RFC3339))

	sql := fmt.Sprintf(`UPDATE heartbeat.heartbeat SET created = '%s'::timestamptz WHERE id = '%s'`,
		createdTime.Format(time.RFC3339), id)
	if err := s.Exec(context.Background(), sql); err != nil {
		t.Fatalf("backdateCreatedOnly exec failed: %v", err)
	}
}

func Test_store_001(t *testing.T) {
	assert := assert.New(t)
	_, err := heartbeat.NewStore(context.Background(), nil)
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
	_, err := s.Create(context.Background(), schema.HeartbeatMeta{Message: "", Schedule: futureSpec(t)})
	assert.Error(err)
}

func Test_store_004(t *testing.T) {
	assert := assert.New(t)
	s := newTestStore(t)
	h, err := s.Create(context.Background(), schema.HeartbeatMeta{Message: "test message", Schedule: futureSpec(t)})
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
	created, err := s.Create(context.Background(), schema.HeartbeatMeta{Message: "hello", Schedule: futureSpec(t)})
	assert.NoError(err)
	got, err := s.Get(context.Background(), created.ID)
	assert.NoError(err)
	assert.Equal(created.ID, got.ID)
	assert.Equal(created.Message, got.Message)
}

func Test_store_007(t *testing.T) {
	assert := assert.New(t)
	s := newTestStore(t)
	_, err := s.Delete(context.Background(), "does-not-exist")
	assert.Error(err)
}

func Test_store_008(t *testing.T) {
	assert := assert.New(t)
	s := newTestStore(t)
	h, err := s.Create(context.Background(), schema.HeartbeatMeta{Message: "bye", Schedule: futureSpec(t)})
	assert.NoError(err)
	deleted, err := s.Delete(context.Background(), h.ID)
	assert.NoError(err)
	assert.Equal(h.ID, deleted.ID)
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
	_, err := s.Create(context.Background(), schema.HeartbeatMeta{Message: "a", Schedule: futureSpec(t)})
	assert.NoError(err)
	_, err = s.Create(context.Background(), schema.HeartbeatMeta{Message: "b", Schedule: futureSpec(t)})
	assert.NoError(err)
	all, err := s.List(context.Background(), true)
	assert.NoError(err)
	assert.GreaterOrEqual(len(all), 2)
}

func Test_store_011(t *testing.T) {
	assert := assert.New(t)
	s := newTestStore(t)
	h, err := s.Create(context.Background(), schema.HeartbeatMeta{Message: "original", Schedule: futureSpec(t)})
	assert.NoError(err)
	updated, err := s.Update(context.Background(), h.ID, schema.HeartbeatMeta{Message: "updated message"})
	assert.NoError(err)
	assert.Equal("updated message", updated.Message)
}

func Test_store_012(t *testing.T) {
	assert := assert.New(t)
	s := newTestStore(t)
	h, err := s.Create(context.Background(), schema.HeartbeatMeta{Message: "one-shot", Schedule: nextMinuteSpec(t)})
	assert.NoError(err)
	backdateHeartbeat(t, s, h.ID)
	fired, err := s.Next(context.Background())
	assert.NoError(err)
	if assert.Len(fired, 1) {
		assert.Equal(h.ID, fired[0].ID)
		assert.True(fired[0].Fired)
	}
	// Rescheduling a fired one-shot should reset fired=false.
	newTs, err := schema.NewTimeSpec("0 9 * * 1-5", nil)
	assert.NoError(err)
	updated, err := s.Update(context.Background(), h.ID, schema.HeartbeatMeta{Schedule: newTs})
	assert.NoError(err)
	assert.False(updated.Fired)
}

func Test_store_013(t *testing.T) {
	assert := assert.New(t)
	s := newTestStore(t)
	_, err := s.Update(context.Background(), "no-such-id", schema.HeartbeatMeta{Message: "msg"})
	assert.Error(err)
}

func Test_store_015(t *testing.T) {
	assert := assert.New(t)
	s := newTestStore(t)
	h, err := s.Create(context.Background(), schema.HeartbeatMeta{Message: "one-shot", Schedule: nextMinuteSpec(t)})
	assert.NoError(err)
	backdateHeartbeat(t, s, h.ID)
	_, err = s.Next(context.Background())
	assert.NoError(err)
	got, err := s.Get(context.Background(), h.ID)
	assert.NoError(err)
	assert.True(got.Fired)
	assert.Nil(got.LastFired)
}

func Test_store_016(t *testing.T) {
	assert := assert.New(t)
	s := newTestStore(t)
	h, err := s.Create(context.Background(), schema.HeartbeatMeta{Message: "recurring", Schedule: futureSpec(t)})
	assert.NoError(err)
	backdateCreatedOnly(t, s, h.ID) // Don't change the recurring schedule
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
	s := newTestStore(t)
	h, err := s.Create(context.Background(), schema.HeartbeatMeta{Message: "fired", Schedule: nextMinuteSpec(t)})
	assert.NoError(err)
	backdateHeartbeat(t, s, h.ID)
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
