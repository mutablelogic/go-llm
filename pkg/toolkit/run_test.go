package toolkit

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
)

///////////////////////////////////////////////////////////////////////////////
// MOCK TYPES

// mockRunConnector implements llm.Connector. It blocks in Run until either
// ctx is cancelled or its disconnect channel is closed (simulating a server
// disconnect). The number of Run calls is recorded in runs.
type mockRunConnector struct {
	mu         sync.Mutex
	runs       int
	disconnect chan struct{}
}

func newMockRunConnector() *mockRunConnector {
	return &mockRunConnector{disconnect: make(chan struct{})}
}

func (m *mockRunConnector) Runs() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.runs
}

func (m *mockRunConnector) Run(ctx context.Context) error {
	m.mu.Lock()
	m.runs++
	disconnect := m.disconnect
	m.mu.Unlock()

	select {
	case <-disconnect:
		// Simulate an unexpected server disconnect; reset the channel so future
		// Run calls block again until the next manual disconnect.
		m.mu.Lock()
		m.disconnect = make(chan struct{})
		m.mu.Unlock()
		return errors.New("disconnected")
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (m *mockRunConnector) ListTools(context.Context) ([]llm.Tool, error)     { return nil, nil }
func (m *mockRunConnector) ListPrompts(context.Context) ([]llm.Prompt, error) { return nil, nil }
func (m *mockRunConnector) ListResources(context.Context) ([]llm.Resource, error) {
	return nil, nil
}

// mockRunHandler implements ToolkitHandler. CreateConnector always returns the
// same mockRunConnector so tests can inspect it after the fact.
type mockRunHandler struct {
	conn *mockRunConnector
}

func (h *mockRunHandler) CreateConnector(_ string, _ func(schema.ConnectorState)) (llm.Connector, error) {
	return h.conn, nil
}
func (h *mockRunHandler) OnStateChange(llm.Connector, schema.ConnectorState) {}
func (h *mockRunHandler) OnToolListChanged(llm.Connector)                    {}
func (h *mockRunHandler) OnPromptListChanged(llm.Connector)                  {}
func (h *mockRunHandler) OnResourceListChanged(llm.Connector)                {}
func (h *mockRunHandler) OnResourceUpdated(llm.Connector, string)            {}
func (h *mockRunHandler) Call(_ context.Context, _ llm.Prompt, _ ...llm.Resource) (llm.Resource, error) {
	return nil, nil
}
func (h *mockRunHandler) List(_ context.Context, _ ListRequest) (*ListResponse, error) {
	return nil, nil
}

///////////////////////////////////////////////////////////////////////////////
// HELPERS

// newRunToolkit creates a toolkit wired up with a mockRunHandler containing
// the given connector.
func newRunToolkit(t *testing.T, conn *mockRunConnector) *toolkit {
	t.Helper()
	tk, err := New(WithHandler(&mockRunHandler{conn: conn}))
	if err != nil {
		t.Fatal(err)
	}
	return tk
}

///////////////////////////////////////////////////////////////////////////////
// Run

// Test_Run_001: cancelling ctx stops all connectors and Run returns ctx.Err().
func Test_Run_001(t *testing.T) {
	conn := newMockRunConnector()
	tk := newRunToolkit(t, conn)

	if err := tk.AddConnector("http://localhost:8080"); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() { done <- tk.Run(ctx) }()

	// Give Run a moment to start the connector.
	time.Sleep(250 * time.Millisecond)
	if conn.Runs() != 1 {
		t.Fatalf("expected connector to be running (runs=1), got runs=%d", conn.Runs())
	}

	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

// Test_Run_002: when a connector's Run returns an error it is restarted on the
// next tick.
func Test_Run_002(t *testing.T) {
	conn := newMockRunConnector()
	tk := newRunToolkit(t, conn)

	if err := tk.AddConnector("http://localhost:8080"); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() { _ = tk.Run(ctx) }()

	// Wait for the first Run call.
	time.Sleep(250 * time.Millisecond)
	if conn.Runs() != 1 {
		t.Fatalf("expected runs=1 after start, got %d", conn.Runs())
	}

	// Trigger a disconnect; the goroutine's Run returns an error.
	close(conn.disconnect)

	// Wait for the reconnect tick (100 ms) plus margin.
	time.Sleep(400 * time.Millisecond)
	if conn.Runs() < 2 {
		t.Fatalf("expected reconnect (runs>=2), got runs=%d", conn.Runs())
	}
}

// Test_Run_003: a connector added after Run is already running is started on
// the next tick.
func Test_Run_003(t *testing.T) {
	conn := newMockRunConnector()
	tk := newRunToolkit(t, conn)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() { _ = tk.Run(ctx) }()

	// Add the connector after Run has started.
	time.Sleep(200 * time.Millisecond)
	if err := tk.AddConnector("http://localhost:9090"); err != nil {
		t.Fatal(err)
	}

	// Wait for the next tick to pick it up.
	time.Sleep(300 * time.Millisecond)
	if conn.Runs() != 1 {
		t.Fatalf("expected connector started after add (runs=1), got runs=%d", conn.Runs())
	}
}

// Test_Run_004: RemoveConnector while Run is active stops the connector and it
// is not restarted.
func Test_Run_004(t *testing.T) {
	conn := newMockRunConnector()
	tk := newRunToolkit(t, conn)

	if err := tk.AddConnector("http://localhost:8080"); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() { _ = tk.Run(ctx) }()

	// Wait for the connector to start.
	time.Sleep(250 * time.Millisecond)
	if conn.Runs() != 1 {
		t.Fatalf("expected runs=1 after start, got %d", conn.Runs())
	}

	// Remove the connector; this cancels it and removes it from the map.
	if err := tk.RemoveConnector("http://localhost:8080"); err != nil {
		t.Fatal(err)
	}

	// Wait several ticks to confirm it is not restarted.
	time.Sleep(400 * time.Millisecond)
	if conn.Runs() != 1 {
		t.Fatalf("expected connector not restarted after remove (runs=1), got runs=%d", conn.Runs())
	}
}
