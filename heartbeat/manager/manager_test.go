package heartbeat_test

import (
	"context"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"

	// Packages
	heartbeat "github.com/mutablelogic/go-llm/heartbeat/manager"
	schema "github.com/mutablelogic/go-llm/heartbeat/schema"
	assert "github.com/stretchr/testify/assert"
)

///////////////////////////////////////////////////////////////////////////////
// MOCK STORE

type mockStore struct {
	heartbeats []*schema.Heartbeat
	nextResult []*schema.Heartbeat
	nextErr    error
}

func (m *mockStore) Create(_ context.Context, meta schema.HeartbeatMeta) (*schema.Heartbeat, error) {
	h := &schema.Heartbeat{
		HeartbeatMeta: meta,
		ID:            "test-id",
		Created:       time.Now(),
	}
	m.heartbeats = append(m.heartbeats, h)
	return h, nil
}

func (m *mockStore) Get(_ context.Context, id string) (*schema.Heartbeat, error) {
	for _, h := range m.heartbeats {
		if h.ID == id {
			return h, nil
		}
	}
	return nil, nil
}

func (m *mockStore) Delete(_ context.Context, id string) (*schema.Heartbeat, error) {
	for i, h := range m.heartbeats {
		if h.ID == id {
			deleted := m.heartbeats[i]
			m.heartbeats = append(m.heartbeats[:i], m.heartbeats[i+1:]...)
			return deleted, nil
		}
	}
	return nil, nil
}

func (m *mockStore) List(_ context.Context, _ bool) ([]*schema.Heartbeat, error) {
	return m.heartbeats, nil
}

func (m *mockStore) Update(_ context.Context, id string, meta schema.HeartbeatMeta) (*schema.Heartbeat, error) {
	for _, h := range m.heartbeats {
		if h.ID == id {
			h.HeartbeatMeta = meta
			return h, nil
		}
	}
	return nil, nil
}

func (m *mockStore) Next(_ context.Context) ([]*schema.Heartbeat, error) {
	if m.nextErr != nil {
		return nil, m.nextErr
	}
	return m.nextResult, nil
}

///////////////////////////////////////////////////////////////////////////////
// New TESTS

func Test_New_001(t *testing.T) {
	// nil store returns error
	assert := assert.New(t)
	mgr, err := heartbeat.New(nil)
	assert.Error(err)
	assert.Nil(mgr)
}

func Test_New_002(t *testing.T) {
	// valid store returns manager
	assert := assert.New(t)
	store := &mockStore{}
	mgr, err := heartbeat.New(store)
	assert.NoError(err)
	assert.NotNil(mgr)
}

func Test_New_003(t *testing.T) {
	// with options
	assert := assert.New(t)
	store := &mockStore{}
	var fired int32
	mgr, err := heartbeat.New(store,
		heartbeat.WithPollInterval(100*time.Millisecond),
		heartbeat.WithLogger(slog.Default()),
		heartbeat.WithOnFire(func(_ context.Context, _ *schema.Heartbeat) {
			atomic.AddInt32(&fired, 1)
		}),
	)
	assert.NoError(err)
	assert.NotNil(mgr)
}

func Test_New_004(t *testing.T) {
	// invalid poll interval
	assert := assert.New(t)
	store := &mockStore{}
	mgr, err := heartbeat.New(store,
		heartbeat.WithPollInterval(0),
	)
	assert.Error(err)
	assert.Nil(mgr)
}

func Test_New_005(t *testing.T) {
	// negative poll interval
	assert := assert.New(t)
	store := &mockStore{}
	mgr, err := heartbeat.New(store,
		heartbeat.WithPollInterval(-1*time.Second),
	)
	assert.Error(err)
	assert.Nil(mgr)
}

func Test_New_006(t *testing.T) {
	// nil logger returns error
	assert := assert.New(t)
	store := &mockStore{}
	mgr, err := heartbeat.New(store,
		heartbeat.WithLogger(nil),
	)
	assert.Error(err)
	assert.Nil(mgr)
}

func Test_New_007(t *testing.T) {
	// nil onFire returns error
	assert := assert.New(t)
	store := &mockStore{}
	mgr, err := heartbeat.New(store,
		heartbeat.WithOnFire(nil),
	)
	assert.Error(err)
	assert.Nil(mgr)
}

///////////////////////////////////////////////////////////////////////////////
// ListTools TESTS

func Test_ListTools_001(t *testing.T) {
	// returns 4 tools
	assert := assert.New(t)
	store := &mockStore{}
	mgr, err := heartbeat.New(store)
	assert.NoError(err)

	tools, err := mgr.ListTools(context.Background())
	assert.NoError(err)
	assert.Len(tools, 4)

	names := make([]string, len(tools))
	for i, tool := range tools {
		names[i] = tool.Name()
	}
	assert.Contains(names, "add_heartbeat")
	assert.Contains(names, "delete_heartbeat")
	assert.Contains(names, "list_heartbeats")
	assert.Contains(names, "update_heartbeat")
}

///////////////////////////////////////////////////////////////////////////////
// ListPrompts TESTS

func Test_ListPrompts_001(t *testing.T) {
	// returns nil
	assert := assert.New(t)
	store := &mockStore{}
	mgr, err := heartbeat.New(store)
	assert.NoError(err)

	prompts, err := mgr.ListPrompts(context.Background())
	assert.NoError(err)
	assert.Nil(prompts)
}

///////////////////////////////////////////////////////////////////////////////
// ListResources TESTS

func Test_ListResources_001(t *testing.T) {
	// returns nil
	assert := assert.New(t)
	store := &mockStore{}
	mgr, err := heartbeat.New(store)
	assert.NoError(err)

	resources, err := mgr.ListResources(context.Background())
	assert.NoError(err)
	assert.Nil(resources)
}

///////////////////////////////////////////////////////////////////////////////
// Run TESTS

func Test_Run_001(t *testing.T) {
	// Run exits when context is cancelled
	assert := assert.New(t)
	store := &mockStore{}
	mgr, err := heartbeat.New(store, heartbeat.WithPollInterval(10*time.Millisecond))
	assert.NoError(err)

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() {
		done <- mgr.Run(ctx)
	}()

	// Let it run briefly
	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		assert.NoError(err)
	case <-time.After(time.Second):
		t.Fatal("Run did not exit after context cancellation")
	}
}

func Test_Run_002(t *testing.T) {
	// Run fires callback for due heartbeats
	assert := assert.New(t)

	ts, _ := schema.NewTimeSpec("* * * * *", nil)
	store := &mockStore{
		nextResult: []*schema.Heartbeat{
			{ID: "h1", HeartbeatMeta: schema.HeartbeatMeta{Message: "test1", Schedule: ts}},
			{ID: "h2", HeartbeatMeta: schema.HeartbeatMeta{Message: "test2", Schedule: ts}},
		},
	}

	var fired int32
	mgr, err := heartbeat.New(store,
		heartbeat.WithPollInterval(10*time.Millisecond),
		heartbeat.WithOnFire(func(_ context.Context, h *schema.Heartbeat) {
			atomic.AddInt32(&fired, 1)
		}),
	)
	assert.NoError(err)

	ctx, cancel := context.WithCancel(context.Background())

	go mgr.Run(ctx)

	// Wait for at least one tick
	time.Sleep(50 * time.Millisecond)
	cancel()

	// Should have fired at least twice (2 heartbeats per tick, at least 1 tick)
	assert.GreaterOrEqual(atomic.LoadInt32(&fired), int32(2))
}

func Test_Run_003(t *testing.T) {
	// Run handles empty next result
	assert := assert.New(t)

	store := &mockStore{
		nextResult: []*schema.Heartbeat{},
	}

	var fired int32
	mgr, err := heartbeat.New(store,
		heartbeat.WithPollInterval(10*time.Millisecond),
		heartbeat.WithOnFire(func(_ context.Context, _ *schema.Heartbeat) {
			atomic.AddInt32(&fired, 1)
		}),
	)
	assert.NoError(err)

	ctx, cancel := context.WithCancel(context.Background())

	go mgr.Run(ctx)
	time.Sleep(50 * time.Millisecond)
	cancel()

	assert.Equal(int32(0), atomic.LoadInt32(&fired))
}
