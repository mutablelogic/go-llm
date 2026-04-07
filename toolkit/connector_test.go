package toolkit

import (
	"context"
	"errors"
	"testing"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/kernel/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// MOCK - connector that records List calls

type mockListConnector struct {
	tools     []llm.Tool
	prompts   []llm.Prompt
	resources []llm.Resource
	runErr    error
	listErr   error
}

func (m *mockListConnector) Run(ctx context.Context) error {
	<-ctx.Done()
	if m.runErr != nil {
		return m.runErr
	}
	return ctx.Err()
}
func (m *mockListConnector) ListTools(_ context.Context) ([]llm.Tool, error) {
	return m.tools, m.listErr
}
func (m *mockListConnector) ListPrompts(_ context.Context) ([]llm.Prompt, error) {
	return m.prompts, m.listErr
}
func (m *mockListConnector) ListResources(_ context.Context) ([]llm.Resource, error) {
	return m.resources, m.listErr
}

// mockConnectorHandler creates a new mockListConnector on each CreateConnector call.
type mockConnectorHandler struct {
	conn *mockListConnector
	err  error
}

func (h *mockConnectorHandler) CreateConnector(_ string, _ func(ConnectorEvent)) (llm.Connector, error) {
	if h.err != nil {
		return nil, h.err
	}
	return h.conn, nil
}
func (h *mockConnectorHandler) OnEvent(ConnectorEvent) {}
func (h *mockConnectorHandler) Call(_ context.Context, _ llm.Prompt, _ ...llm.Resource) (llm.Resource, error) {
	return nil, nil
}
func (h *mockConnectorHandler) List(_ context.Context, _ ListRequest) (*ListResponse, error) {
	return nil, nil
}

// newConnectorToolkit creates a toolkit with a real mockListConnector wired in.
func newConnectorToolkit(t *testing.T) (*toolkit, *mockListConnector) {
	t.Helper()
	conn := &mockListConnector{}
	tk, err := New(WithDelegate(&mockConnectorHandler{conn: conn}))
	if err != nil {
		t.Fatal(err)
	}
	return tk, conn
}

///////////////////////////////////////////////////////////////////////////////
// canonicalURL

func Test_canonicalURL_001(t *testing.T) {
	got, err := canonicalURL("http://localhost:8080/path")
	if err != nil {
		t.Fatal(err)
	}
	if got != "http://localhost:8080/path" {
		t.Fatalf("unexpected: %q", got)
	}
}

func Test_canonicalURL_002_default_port_stripped(t *testing.T) {
	got, err := canonicalURL("http://localhost:80/path")
	if err != nil {
		t.Fatal(err)
	}
	if got != "http://localhost/path" {
		t.Fatalf("unexpected: %q", got)
	}
}

func Test_canonicalURL_003_https_default_port(t *testing.T) {
	got, err := canonicalURL("https://example.com:443/")
	if err != nil {
		t.Fatal(err)
	}
	if got != "https://example.com/" {
		t.Fatalf("unexpected: %q", got)
	}
}

func Test_canonicalURL_004_scheme_host_lowercased_path_preserved(t *testing.T) {
	// Scheme and host are normalised to lower-case; path case is preserved.
	got, err := canonicalURL("HTTP://EXAMPLE.COM/MyPath")
	if err != nil {
		t.Fatal(err)
	}
	if got != "http://example.com/MyPath" {
		t.Fatalf("unexpected: %q", got)
	}
}

func Test_canonicalURL_005_trailing_slash_preserved(t *testing.T) {
	got, err := canonicalURL("http://example.com/path/")
	if err != nil {
		t.Fatal(err)
	}
	if got != "http://example.com/path/" {
		t.Fatalf("unexpected: %q", got)
	}
}

func Test_canonicalURL_006_userinfo_fragment_query_stripped(t *testing.T) {
	got, err := canonicalURL("http://user:pass@example.com/path?q=1#frag")
	if err != nil {
		t.Fatal(err)
	}
	if got != "http://example.com/path" {
		t.Fatalf("unexpected: %q", got)
	}
}

func Test_canonicalURL_007_no_path(t *testing.T) {
	got, err := canonicalURL("http://example.com")
	if err != nil {
		t.Fatal(err)
	}
	if got != "http://example.com" {
		t.Fatalf("unexpected: %q", got)
	}
}

func Test_canonicalURL_008_relative_url_error(t *testing.T) {
	_, err := canonicalURL("example.com/path")
	if !errors.Is(err, schema.ErrBadParameter) {
		t.Fatalf("expected ErrBadParameter, got %v", err)
	}
}

func Test_canonicalURL_009_unsupported_scheme(t *testing.T) {
	_, err := canonicalURL("ftp://example.com/path")
	if !errors.Is(err, schema.ErrBadParameter) {
		t.Fatalf("expected ErrBadParameter, got %v", err)
	}
}

func Test_canonicalURL_010_missing_host(t *testing.T) {
	_, err := canonicalURL("http:///path")
	if !errors.Is(err, schema.ErrBadParameter) {
		t.Fatalf("expected ErrBadParameter, got %v", err)
	}
}

func Test_canonicalURL_011_invalid_port(t *testing.T) {
	_, err := canonicalURL("http://example.com:99999/path")
	if !errors.Is(err, schema.ErrBadParameter) {
		t.Fatalf("expected ErrBadParameter, got %v", err)
	}
}

func Test_canonicalURL_012_port_zero_error(t *testing.T) {
	_, err := canonicalURL("http://example.com:0/path")
	if !errors.Is(err, schema.ErrBadParameter) {
		t.Fatalf("expected ErrBadParameter, got %v", err)
	}
}

///////////////////////////////////////////////////////////////////////////////
// AddConnector / AddConnectorNS

func Test_AddConnector_001(t *testing.T) {
	tk, _ := newConnectorToolkit(t)
	if err := tk.AddConnector("http://localhost:8080"); err != nil {
		t.Fatal(err)
	}
	if len(tk.connectors) != 1 {
		t.Fatalf("expected 1 connector, got %d", len(tk.connectors))
	}
}

func Test_AddConnector_002_duplicate(t *testing.T) {
	tk, _ := newConnectorToolkit(t)
	if err := tk.AddConnector("http://localhost:8080"); err != nil {
		t.Fatal(err)
	}
	if err := tk.AddConnector("http://localhost:8080"); !errors.Is(err, schema.ErrConflict) {
		t.Fatalf("expected ErrConflict, got %v", err)
	}
}

func Test_ExistsConnector_001_canonical_url(t *testing.T) {
	tk, _ := newConnectorToolkit(t)
	if err := tk.AddConnector("http://LOCALHOST:80/path?q=1#frag"); err != nil {
		t.Fatal(err)
	}
	if !tk.ExistsConnector("http://localhost/path") {
		t.Fatal("expected connector to exist for canonical URL")
	}
	if !tk.ExistsConnector("http://localhost:80/path") {
		t.Fatal("expected connector to exist for equivalent URL")
	}
	if tk.ExistsConnector("http://localhost/other") {
		t.Fatal("did not expect different canonical URL to exist")
	}
}

func Test_AddConnector_003_no_handler(t *testing.T) {
	tk, _ := New()
	if err := tk.AddConnector("http://localhost:8080"); !errors.Is(err, schema.ErrNotImplemented) {
		t.Fatalf("expected ErrNotImplemented, got %v", err)
	}
}

func Test_AddConnector_004_bad_url(t *testing.T) {
	tk, _ := newConnectorToolkit(t)
	if err := tk.AddConnector("not-a-url"); !errors.Is(err, schema.ErrBadParameter) {
		t.Fatalf("expected ErrBadParameter, got %v", err)
	}
}

func Test_AddConnector_005_handler_error(t *testing.T) {
	sentinel := errors.New("handler failure")
	tk, err := New(WithDelegate(&mockConnectorHandler{err: sentinel}))
	if err != nil {
		t.Fatal(err)
	}
	if got := tk.AddConnector("http://localhost:8080"); !errors.Is(got, sentinel) {
		t.Fatalf("expected sentinel error, got %v", got)
	}
}

func Test_AddConnectorNS_001(t *testing.T) {
	tk, _ := newConnectorToolkit(t)
	if err := tk.AddConnectorNS("mynamespace", "http://localhost:9090"); err != nil {
		t.Fatal(err)
	}
	c := tk.connectors["http://localhost:9090"]
	if c == nil {
		t.Fatal("connector not stored")
	}
	if c.namespace != "mynamespace" {
		t.Fatalf("expected namespace %q, got %q", "mynamespace", c.namespace)
	}
}

///////////////////////////////////////////////////////////////////////////////
// RemoveConnector

func Test_RemoveConnector_001(t *testing.T) {
	tk, _ := newConnectorToolkit(t)
	if err := tk.AddConnector("http://localhost:8080"); err != nil {
		t.Fatal(err)
	}
	if err := tk.RemoveConnector("http://localhost:8080"); err != nil {
		t.Fatal(err)
	}
	if len(tk.connectors) != 0 {
		t.Fatalf("expected 0 connectors, got %d", len(tk.connectors))
	}
}

func Test_RemoveConnector_002_not_found(t *testing.T) {
	tk, _ := newConnectorToolkit(t)
	if err := tk.RemoveConnector("http://localhost:8080"); !errors.Is(err, schema.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func Test_RemoveConnector_003_bad_url(t *testing.T) {
	tk, _ := newConnectorToolkit(t)
	if err := tk.RemoveConnector("not-a-url"); !errors.Is(err, schema.ErrBadParameter) {
		t.Fatalf("expected ErrBadParameter, got %v", err)
	}
}

func Test_RemoveConnector_004_url_normalised(t *testing.T) {
	// Scheme and host are normalised; path casing is preserved, so add/remove
	// must use the same path case to resolve to the same canonical key.
	tk, _ := newConnectorToolkit(t)
	if err := tk.AddConnector("http://localhost:8080/path"); err != nil {
		t.Fatal(err)
	}
	if err := tk.RemoveConnector("HTTP://LOCALHOST:8080/path"); err != nil {
		t.Fatal(err)
	}
}

///////////////////////////////////////////////////////////////////////////////
// connector delegate methods

func Test_connector_delegates_001(t *testing.T) {
	ctx := context.Background()
	inner := &mockListConnector{
		tools:     []llm.Tool{&mockTool{name: "t1"}},
		prompts:   []llm.Prompt{&mockPrompt{name: "p1"}},
		resources: []llm.Resource{},
	}
	c := &connector{conn: inner}

	tools, err := c.ListTools(ctx)
	if err != nil || len(tools) != 1 {
		t.Fatalf("ListTools: %v, %v", tools, err)
	}
	prompts, err := c.ListPrompts(ctx)
	if err != nil || len(prompts) != 1 {
		t.Fatalf("ListPrompts: %v, %v", prompts, err)
	}
	resources, err := c.ListResources(ctx)
	if err != nil || len(resources) != 0 {
		t.Fatalf("ListResources: %v, %v", resources, err)
	}
}

func Test_connector_Run_001(t *testing.T) {
	inner := &mockListConnector{}
	c := &connector{conn: inner}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- c.Run(ctx) }()

	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

///////////////////////////////////////////////////////////////////////////////
// onConnectorEvent

// newTestConnector builds a bare connector struct for unit tests (no goroutine).
func newTestConnector(ns string) *connector {
	return &connector{namespace: ns}
}

func Test_onConnectorEvent_001_state_change_sets_namespace(t *testing.T) {
	tk, _ := New()
	c := newTestConnector("")
	state := schema.ConnectorState{Name: types.Ptr("myserver"), Version: types.Ptr("1.0")}
	tk.onConnectorEvent(c, StateChangeEvent(state))
	if c.namespace != "myserver" {
		t.Fatalf("expected namespace %q, got %q", "myserver", c.namespace)
	}
	if tk.namespace["myserver"] != c {
		t.Fatal("expected connector registered in namespace map")
	}
}

func Test_onConnectorEvent_002_state_change_preserves_existing_namespace(t *testing.T) {
	// When a namespace is already set, the server-reported name must not override it.
	tk, _ := New()
	c := newTestConnector("pinned")
	tk.namespace["pinned"] = c
	state := schema.ConnectorState{Name: types.Ptr("othername"), Version: types.Ptr("1.0")}
	tk.onConnectorEvent(c, StateChangeEvent(state))
	if c.namespace != "pinned" {
		t.Fatalf("expected namespace %q unchanged, got %q", "pinned", c.namespace)
	}
}

func Test_onConnectorEvent_003_state_change_invalid_name(t *testing.T) {
	// A server reporting an invalid identifier sets c.err and does not register.
	tk, _ := New()
	c := newTestConnector("")
	state := schema.ConnectorState{Name: types.Ptr("bad name!")}
	tk.onConnectorEvent(c, StateChangeEvent(state))
	if c.err == nil {
		t.Fatal("expected error for invalid namespace name")
	}
}

func Test_onConnectorEvent_004_state_change_reserved_name(t *testing.T) {
	tk, _ := New()
	c := newTestConnector("")
	state := schema.ConnectorState{Name: types.Ptr("builtin")}
	tk.onConnectorEvent(c, StateChangeEvent(state))
	if c.err == nil {
		t.Fatal("expected error for reserved namespace")
	}
}

func Test_onConnectorEvent_005_state_change_namespace_collision(t *testing.T) {
	tk, _ := New()
	other := newTestConnector("taken")
	tk.namespace["taken"] = other
	c := newTestConnector("")
	state := schema.ConnectorState{Name: types.Ptr("taken")}
	tk.onConnectorEvent(c, StateChangeEvent(state))
	if c.err == nil {
		t.Fatal("expected conflict error for colliding namespace")
	}
}

func Test_onConnectorEvent_006_state_change_no_name_no_namespace(t *testing.T) {
	// If server sends no name and connector has no namespace, event is silently ignored.
	tk, _ := New()
	c := newTestConnector("")
	state := schema.ConnectorState{}
	tk.onConnectorEvent(c, StateChangeEvent(state))
	if c.namespace != "" {
		t.Fatalf("expected empty namespace, got %q", c.namespace)
	}
}

func Test_onConnectorEvent_007_non_state_forwarded_to_delegate(t *testing.T) {
	d := &mockDelegate{}
	tk, _ := New(WithDelegate(d))
	c := newTestConnector("mymcp")
	tk.onConnectorEvent(c, ToolListChangeEvent())
	if len(d.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(d.events))
	}
	if d.events[0].Kind != ConnectorEventToolListChanged {
		t.Fatalf("expected ToolListChanged, got %v", d.events[0].Kind)
	}
	// Connector field must be injected by the toolkit.
	if d.events[0].Connector != c {
		t.Fatalf("expected Connector field to be set")
	}
}

func Test_onConnectorEvent_008_non_state_no_delegate(t *testing.T) {
	// Without a delegate, non-StateChange events are silently dropped.
	tk, _ := New()
	c := newTestConnector("mymcp")
	// Must not panic.
	tk.onConnectorEvent(c, ToolListChangeEvent())
}

func Test_onConnectorEvent_009_disconnected_forwarded_to_delegate(t *testing.T) {
	d := &mockDelegate{}
	tk, _ := New(WithDelegate(d))
	c := newTestConnector("mymcp")
	err := errors.New("boom")
	tk.onConnectorEvent(c, DisconnectedEvent(err))
	if len(d.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(d.events))
	}
	if d.events[0].Kind != ConnectorEventDisconnected {
		t.Fatalf("expected Disconnected, got %v", d.events[0].Kind)
	}
	if !errors.Is(d.events[0].Err, err) {
		t.Fatalf("expected disconnect error %v, got %v", err, d.events[0].Err)
	}
	if d.events[0].Connector != c {
		t.Fatalf("expected Connector field to be set")
	}
}
