package manager

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	llmtest "github.com/mutablelogic/go-llm/pkg/test"
	toolkit "github.com/mutablelogic/go-llm/pkg/toolkit"
	pg "github.com/mutablelogic/go-pg"
	jsonschema "github.com/mutablelogic/go-server/pkg/jsonschema"
	types "github.com/mutablelogic/go-server/pkg/types"
)

type listToolsMockTool struct {
	name        string
	description string
	input       *jsonschema.Schema
	output      *jsonschema.Schema
	meta        llm.ToolMeta
}

func (t *listToolsMockTool) Name() string                     { return t.name }
func (t *listToolsMockTool) Description() string              { return t.description }
func (t *listToolsMockTool) InputSchema() *jsonschema.Schema  { return t.input }
func (t *listToolsMockTool) OutputSchema() *jsonschema.Schema { return t.output }
func (t *listToolsMockTool) Meta() llm.ToolMeta               { return t.meta }
func (t *listToolsMockTool) Run(context.Context, json.RawMessage) (any, error) {
	return nil, nil
}

func TestListTools(t *testing.T) {
	m := newListToolsManager(t)
	limit := uint64(2)
	resp, err := m.ListTools(context.Background(), schema.ToolListRequest{
		OffsetLimit: pg.OffsetLimit{Limit: &limit},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Count != 3 {
		t.Fatalf("expected Count=3, got %d", resp.Count)
	}
	if len(resp.Body) != 2 {
		t.Fatalf("expected 2 tools after pagination, got %d", len(resp.Body))
	}
	if resp.Body[0].Name != "builtin.alpha" {
		t.Fatalf("expected first tool %q, got %q", "builtin.alpha", resp.Body[0].Name)
	}
	if resp.Body[1].Name != "builtin.bravo" {
		t.Fatalf("expected second tool %q, got %q", "builtin.bravo", resp.Body[1].Name)
	}
	if string(resp.Body[0].Input) == "" {
		t.Fatal("expected first tool to include input schema")
	}
	if string(resp.Body[0].Output) == "" {
		t.Fatal("expected first tool to include output schema")
	}
	if resp.Body[0].Title != "Alpha Tool" {
		t.Fatalf("expected title %q, got %q", "Alpha Tool", resp.Body[0].Title)
	}
	if resp.Body[0].Hints == nil {
		t.Fatal("expected first tool to include hints")
	}
	if len(resp.Body[0].Hints) != 2 || resp.Body[0].Hints[0] != "readonly" || resp.Body[0].Hints[1] != "idempotent" {
		t.Fatalf("unexpected hint keywords: %+v", resp.Body[0].Hints)
	}
}

func TestGetTool(t *testing.T) {
	m := newListToolsManager(t)

	meta, err := m.GetTool(context.Background(), "builtin.alpha", nil)
	if err != nil {
		t.Fatal(err)
	}
	if meta.Name != "builtin.alpha" {
		t.Fatalf("expected tool %q, got %q", "builtin.alpha", meta.Name)
	}
	if meta.Title != "Alpha Tool" {
		t.Fatalf("expected title %q, got %q", "Alpha Tool", meta.Title)
	}
	if string(meta.Input) == "" || string(meta.Output) == "" {
		t.Fatal("expected tool schemas in metadata")
	}
}

func TestGetToolNotFound(t *testing.T) {
	m := newListToolsManager(t)

	if _, err := m.GetTool(context.Background(), "builtin.missing", nil); err == nil {
		t.Fatal("expected not found error, got nil")
	}
}

func TestListToolsWithNameFilters(t *testing.T) {
	m := newListToolsManager(t)

	resp, err := m.ListTools(context.Background(), schema.ToolListRequest{
		Name: []string{"builtin.charlie", "builtin.alpha", "builtin.alpha"},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Count != 2 {
		t.Fatalf("expected Count=2, got %d", resp.Count)
	}
	if len(resp.Body) != 2 {
		t.Fatalf("expected 2 tools after filtering, got %d", len(resp.Body))
	}
	if resp.Body[0].Name != "builtin.alpha" || resp.Body[1].Name != "builtin.charlie" {
		t.Fatalf("unexpected filtered tools: %+v", resp.Body)
	}
}

func TestListToolsWithNamespaceFilter(t *testing.T) {
	m := newListToolsManager(t)

	resp, err := m.ListTools(context.Background(), schema.ToolListRequest{Namespace: "missing"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Count != 0 {
		t.Fatalf("expected Count=0, got %d", resp.Count)
	}
	if len(resp.Body) != 0 {
		t.Fatalf("expected 0 tools after namespace filtering, got %d", len(resp.Body))
	}
}

func TestListToolsWithUserScopedNamespaces(t *testing.T) {
	conn, m := newIntegrationManager(t)
	ctx := llmtest.Context(t)

	if err := m.Exec(ctx, `TRUNCATE llm.connector CASCADE`); err != nil {
		t.Fatal(err)
	}
	if err := m.Toolkit.AddTool(&listToolsMockTool{name: "local", description: "builtin"}); err != nil {
		t.Fatal(err)
	}

	publicNamespace := "publictools"
	privateNamespace := "privatetools"
	publicURL := llmtest.ConnectorURL(t, "list-tools-public")
	privateURL := llmtest.ConnectorURL(t, "list-tools-private")
	if _, _, _, err := m.CreateConnector(ctx, schema.ConnectorInsert{
		URL:           publicURL,
		ConnectorMeta: schema.ConnectorMeta{Namespace: types.Ptr(publicNamespace)},
	}, nil); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := m.CreateConnector(ctx, schema.ConnectorInsert{
		URL:           privateURL,
		ConnectorMeta: schema.ConnectorMeta{Namespace: types.Ptr(privateNamespace), Groups: conn.Config.Groups},
	}, llmtest.AdminUser(conn)); err != nil {
		t.Fatal(err)
	}

	llmtest.WaitUntil(t, 5*time.Second, func() bool {
		resp, err := m.Toolkit.List(ctx, toolkit.ListRequest{Type: toolkit.ListTypeTools})
		if err != nil {
			return false
		}
		return resp.Count >= 3
	}, "timed out waiting for connector tools to appear in toolkit")

	adminResp, err := m.ListTools(ctx, schema.ToolListRequest{}, llmtest.AdminUser(conn))
	if err != nil {
		t.Fatal(err)
	}
	if adminResp.Count != 3 {
		t.Fatalf("expected admin to see 3 tools, got %d", adminResp.Count)
	}

	userResp, err := m.ListTools(ctx, schema.ToolListRequest{}, llmtest.User(conn))
	if err != nil {
		t.Fatal(err)
	}
	if userResp.Count != 2 {
		t.Fatalf("expected ungrouped user to see 2 tools, got %d", userResp.Count)
	}
	for _, tool := range userResp.Body {
		if tool.Name == privateNamespace+".remote_tool" {
			t.Fatal("expected private tool to be filtered out for ungrouped user")
		}
	}

	privateResp, err := m.ListTools(ctx, schema.ToolListRequest{Namespace: privateNamespace}, llmtest.User(conn))
	if err != nil {
		t.Fatal(err)
	}
	if privateResp.Count != 0 || len(privateResp.Body) != 0 {
		t.Fatalf("expected inaccessible private namespace to return no tools, got count=%d len=%d", privateResp.Count, len(privateResp.Body))
	}
}

func TestGetToolWithUserScopedNamespaces(t *testing.T) {
	conn, m := newIntegrationManager(t)
	ctx := llmtest.Context(t)

	if err := m.Exec(ctx, `TRUNCATE llm.connector CASCADE`); err != nil {
		t.Fatal(err)
	}

	publicNamespace := "publictoolget"
	privateNamespace := "privatetoolget"
	publicURL := llmtest.ConnectorURL(t, "get-tool-public")
	privateURL := llmtest.ConnectorURL(t, "get-tool-private")
	if _, _, _, err := m.CreateConnector(ctx, schema.ConnectorInsert{
		URL:           publicURL,
		ConnectorMeta: schema.ConnectorMeta{Namespace: types.Ptr(publicNamespace)},
	}, nil); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := m.CreateConnector(ctx, schema.ConnectorInsert{
		URL:           privateURL,
		ConnectorMeta: schema.ConnectorMeta{Namespace: types.Ptr(privateNamespace), Groups: conn.Config.Groups},
	}, llmtest.AdminUser(conn)); err != nil {
		t.Fatal(err)
	}

	llmtest.WaitUntil(t, 5*time.Second, func() bool {
		resp, err := m.Toolkit.List(ctx, toolkit.ListRequest{Type: toolkit.ListTypeTools})
		if err != nil {
			return false
		}
		return resp.Count >= 2
	}, "timed out waiting for connector tools to appear in toolkit")

	if _, err := m.GetTool(ctx, publicNamespace+".remote_tool", llmtest.User(conn)); err != nil {
		t.Fatalf("expected public connector tool lookup to succeed: %v", err)
	}
	if _, err := m.GetTool(ctx, privateNamespace+".remote_tool", llmtest.User(conn)); err == nil {
		t.Fatal("expected private connector tool lookup to be denied")
	}
	if _, err := m.GetTool(ctx, privateNamespace+".remote_tool", llmtest.AdminUser(conn)); err != nil {
		t.Fatalf("expected admin private connector tool lookup to succeed: %v", err)
	}
}

func TestToolNamespacesForUserPagesAllConnectors(t *testing.T) {
	conn, m := newIntegrationManager(t)
	ctx := llmtest.Context(t)
	user := llmtest.User(conn)

	if err := m.Exec(ctx, `TRUNCATE llm.connector CASCADE`); err != nil {
		t.Fatal(err)
	}

	total := int(schema.ConnectorListMax) + 5
	for i := range total {
		var inserted schema.Connector
		namespace := fmt.Sprintf("ns%03d", i)
		url := fmt.Sprintf("https://example.com/%03d", i)
		if err := m.Insert(ctx, &inserted, schema.ConnectorInsert{
			URL:           url,
			ConnectorMeta: schema.ConnectorMeta{Namespace: types.Ptr(namespace)},
		}); err != nil {
			t.Fatal(err)
		}
	}

	namespaces, err := m.toolNamespacesForUser(ctx, user)
	if err != nil {
		t.Fatal(err)
	}
	if len(namespaces) != total+1 {
		t.Fatalf("expected %d namespaces including builtin, got %d", total+1, len(namespaces))
	}
	if namespaces[0] != schema.BuiltinNamespace {
		t.Fatalf("expected builtin namespace first, got %q", namespaces[0])
	}
	seen := make(map[string]struct{}, len(namespaces))
	for _, namespace := range namespaces[1:] {
		seen[namespace] = struct{}{}
	}
	for i := 0; i < total; i++ {
		namespace := fmt.Sprintf("ns%03d", i)
		if _, ok := seen[namespace]; !ok {
			t.Fatalf("expected namespace %q to be present", namespace)
		}
	}
}

func newListToolsManager(t *testing.T) *Manager {
	tk, err := toolkit.New()
	if err != nil {
		t.Fatal(err)
	}
	if err := tk.AddTool(
		&listToolsMockTool{name: "charlie", description: "C"},
		&listToolsMockTool{name: "alpha", description: "A", input: jsonschema.MustFor[map[string]any](), output: jsonschema.MustFor[string](), meta: llm.ToolMeta{Title: "Alpha Tool", ReadOnlyHint: true, IdempotentHint: true}},
		&listToolsMockTool{name: "bravo", description: "B"},
	); err != nil {
		t.Fatal(err)
	}

	return &Manager{Toolkit: tk}
}
