package toolkit

import (
	"context"
	"errors"
	"testing"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
)

///////////////////////////////////////////////////////////////////////////////
// MOCK - connector that records List calls

type mockListConnector struct {
	tools     []llm.Tool
	prompts   []llm.Prompt
	resources []llm.Resource
	runErr    error
}

func (m *mockListConnector) Run(ctx context.Context) error {
	<-ctx.Done()
	if m.runErr != nil {
		return m.runErr
	}
	return ctx.Err()
}
func (m *mockListConnector) ListTools(_ context.Context) ([]llm.Tool, error) {
	return m.tools, nil
}
func (m *mockListConnector) ListPrompts(_ context.Context) ([]llm.Prompt, error) {
	return m.prompts, nil
}
func (m *mockListConnector) ListResources(_ context.Context) ([]llm.Resource, error) {
	return m.resources, nil
}

// mockConnectorHandler creates a new mockListConnector on each CreateConnector call.
type mockConnectorHandler struct {
	conn *mockListConnector
	err  error
}

func (h *mockConnectorHandler) CreateConnector(_ string) (llm.Connector, error) {
	if h.err != nil {
		return nil, h.err
	}
	return h.conn, nil
}
func (h *mockConnectorHandler) OnStateChange(llm.Connector, schema.ConnectorState) {}
func (h *mockConnectorHandler) OnToolListChanged(llm.Connector)                    {}
func (h *mockConnectorHandler) OnPromptListChanged(llm.Connector)                  {}
func (h *mockConnectorHandler) OnResourceListChanged(llm.Connector)                {}
func (h *mockConnectorHandler) OnResourceUpdated(llm.Connector, string)            {}
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
	tk, err := New(WithHandler(&mockConnectorHandler{conn: conn}))
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

func Test_canonicalURL_004_uppercased_normalised(t *testing.T) {
	got, err := canonicalURL("HTTP://EXAMPLE.COM/PATH")
	if err != nil {
		t.Fatal(err)
	}
	if got != "http://example.com/path" {
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
	if !errors.Is(err, llm.ErrBadParameter) {
		t.Fatalf("expected ErrBadParameter, got %v", err)
	}
}

func Test_canonicalURL_009_unsupported_scheme(t *testing.T) {
	_, err := canonicalURL("ftp://example.com/path")
	if !errors.Is(err, llm.ErrBadParameter) {
		t.Fatalf("expected ErrBadParameter, got %v", err)
	}
}

func Test_canonicalURL_010_missing_host(t *testing.T) {
	_, err := canonicalURL("http:///path")
	if !errors.Is(err, llm.ErrBadParameter) {
		t.Fatalf("expected ErrBadParameter, got %v", err)
	}
}

func Test_canonicalURL_011_invalid_port(t *testing.T) {
	_, err := canonicalURL("http://example.com:99999/path")
	if !errors.Is(err, llm.ErrBadParameter) {
		t.Fatalf("expected ErrBadParameter, got %v", err)
	}
}

func Test_canonicalURL_012_port_zero_error(t *testing.T) {
	_, err := canonicalURL("http://example.com:0/path")
	if !errors.Is(err, llm.ErrBadParameter) {
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
	if err := tk.AddConnector("http://localhost:8080"); !errors.Is(err, llm.ErrConflict) {
		t.Fatalf("expected ErrConflict, got %v", err)
	}
}

func Test_AddConnector_003_no_handler(t *testing.T) {
	tk, _ := New()
	if err := tk.AddConnector("http://localhost:8080"); !errors.Is(err, llm.ErrNotImplemented) {
		t.Fatalf("expected ErrNotImplemented, got %v", err)
	}
}

func Test_AddConnector_004_bad_url(t *testing.T) {
	tk, _ := newConnectorToolkit(t)
	if err := tk.AddConnector("not-a-url"); !errors.Is(err, llm.ErrBadParameter) {
		t.Fatalf("expected ErrBadParameter, got %v", err)
	}
}

func Test_AddConnector_005_handler_error(t *testing.T) {
	sentinel := errors.New("handler failure")
	tk, err := New(WithHandler(&mockConnectorHandler{err: sentinel}))
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
	if err := tk.RemoveConnector("http://localhost:8080"); !errors.Is(err, llm.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func Test_RemoveConnector_003_bad_url(t *testing.T) {
	tk, _ := newConnectorToolkit(t)
	if err := tk.RemoveConnector("not-a-url"); !errors.Is(err, llm.ErrBadParameter) {
		t.Fatalf("expected ErrBadParameter, got %v", err)
	}
}

func Test_RemoveConnector_004_url_normalised(t *testing.T) {
	// Add with one form, remove with a case-variant — both should resolve to the same key.
	tk, _ := newConnectorToolkit(t)
	if err := tk.AddConnector("http://localhost:8080/path"); err != nil {
		t.Fatal(err)
	}
	if err := tk.RemoveConnector("HTTP://LOCALHOST:8080/PATH"); err != nil {
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
