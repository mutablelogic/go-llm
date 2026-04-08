package manager

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/kernel/schema"
	mcpserver "github.com/mutablelogic/go-llm/mcp/server"
	opt "github.com/mutablelogic/go-llm/pkg/opt"
	llmtest "github.com/mutablelogic/go-llm/pkg/test"
	toolkit "github.com/mutablelogic/go-llm/toolkit"
	resource "github.com/mutablelogic/go-llm/toolkit/resource"
	pg "github.com/mutablelogic/go-pg"
	types "github.com/mutablelogic/go-server/pkg/types"
)

type listAgentsMockPrompt struct {
	name        string
	title       string
	description string
}

type callAgentsMockDelegate struct {
	prompt    llm.Prompt
	resources []llm.Resource
	result    llm.Resource
	err       error
}

func (p *listAgentsMockPrompt) Name() string        { return p.name }
func (p *listAgentsMockPrompt) Title() string       { return p.title }
func (p *listAgentsMockPrompt) Description() string { return p.description }
func (p *listAgentsMockPrompt) Prepare(context.Context, json.RawMessage) (string, []opt.Opt, error) {
	return "", nil, nil
}

func (d *callAgentsMockDelegate) OnEvent(toolkit.ConnectorEvent) {}

func (d *callAgentsMockDelegate) Call(_ context.Context, prompt llm.Prompt, resources ...llm.Resource) (llm.Resource, error) {
	d.prompt = prompt
	d.resources = resources
	return d.result, d.err
}

func (d *callAgentsMockDelegate) CreateConnector(string, func(toolkit.ConnectorEvent)) (llm.Connector, error) {
	return nil, nil
}

func TestListAgents(t *testing.T) {
	m := newListAgentsManager(t)
	limit := uint64(2)
	resp, err := m.ListAgents(context.Background(), schema.AgentListRequest{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Count != 3 {
		t.Fatalf("expected Count=3, got %d", resp.Count)
	}
	if len(resp.Body) != 3 {
		t.Fatalf("expected 3 agents without explicit limit, got %d", len(resp.Body))
	}

	resp, err = m.ListAgents(context.Background(), schema.AgentListRequest{
		OffsetLimit: pg.OffsetLimit{Limit: &limit},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Body) != 2 {
		t.Fatalf("expected 2 agents after pagination, got %d", len(resp.Body))
	}
	if resp.Body[0].Name != "builtin.alpha" {
		t.Fatalf("expected first agent %q, got %q", "builtin.alpha", resp.Body[0].Name)
	}
	if resp.Body[0].Title != "Alpha Agent" {
		t.Fatalf("expected title %q, got %q", "Alpha Agent", resp.Body[0].Title)
	}
	if resp.Body[1].Name != "builtin.bravo" {
		t.Fatalf("expected second agent %q, got %q", "builtin.bravo", resp.Body[1].Name)
	}
}

func TestListAgentsWithNameFilters(t *testing.T) {
	m := newListAgentsManager(t)

	resp, err := m.ListAgents(context.Background(), schema.AgentListRequest{
		Name: []string{"builtin.charlie", "builtin.alpha", "builtin.alpha"},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Count != 2 {
		t.Fatalf("expected Count=2, got %d", resp.Count)
	}
	if len(resp.Body) != 2 {
		t.Fatalf("expected 2 agents after filtering, got %d", len(resp.Body))
	}
	if resp.Body[0].Name != "builtin.alpha" || resp.Body[1].Name != "builtin.charlie" {
		t.Fatalf("unexpected filtered agents: %+v", resp.Body)
	}
}

func TestGetAgent(t *testing.T) {
	m := newListAgentsManager(t)

	meta, err := m.GetAgent(context.Background(), "builtin.alpha", nil)
	if err != nil {
		t.Fatal(err)
	}
	if meta.Name != "builtin.alpha" {
		t.Fatalf("expected agent %q, got %q", "builtin.alpha", meta.Name)
	}
	if meta.Title != "Alpha Agent" {
		t.Fatalf("expected title %q, got %q", "Alpha Agent", meta.Title)
	}
}

func TestGetAgentBareName(t *testing.T) {
	m := newListAgentsManager(t)

	meta, err := m.GetAgent(context.Background(), "alpha", nil)
	if err != nil {
		t.Fatal(err)
	}
	if meta.Name != "builtin.alpha" {
		t.Fatalf("expected agent %q, got %q", "builtin.alpha", meta.Name)
	}
}

func TestGetAgentNotFound(t *testing.T) {
	m := newListAgentsManager(t)

	if _, err := m.GetAgent(context.Background(), "builtin.missing", nil); err == nil {
		t.Fatal("expected not found error, got nil")
	}
}

func TestCallAgent(t *testing.T) {
	delegate := &callAgentsMockDelegate{}
	tk, err := toolkit.New(toolkit.WithDelegate(delegate))
	if err != nil {
		t.Fatal(err)
	}
	if err := tk.AddPrompt(&listAgentsMockPrompt{name: "alpha", title: "Alpha Agent", description: "A"}); err != nil {
		t.Fatal(err)
	}
	want := resource.Must("alpha", "hello")
	attachment := resource.Must("note", "attached")
	delegate.result = want
	m := &Manager{Toolkit: tk}
	req := schema.CallAgentRequest{
		CallToolRequest: schema.CallToolRequest{Input: json.RawMessage(`{"topic":"docs"}`)},
		Attachments: []interface {
			URI() string
			Name() string
			Description() string
			Type() string
			Read(context.Context) ([]byte, error)
		}{attachment},
	}

	resp, err := m.CallAgent(context.Background(), "builtin.alpha", req, nil)
	if err != nil {
		t.Fatal(err)
	}
	if resp == nil {
		t.Fatal("expected agent resource, got nil")
	}
	if delegate.prompt == nil || delegate.prompt.Name() != "builtin.alpha" {
		t.Fatalf("expected delegate prompt %q, got %#v", "builtin.alpha", delegate.prompt)
	}
	if len(delegate.resources) != 2 {
		t.Fatalf("expected 2 resources, got %d", len(delegate.resources))
	}
	if delegate.resources[0].Type() != "application/json" {
		t.Fatalf("expected input resource type %q, got %q", "application/json", delegate.resources[0].Type())
	}
	data, err := delegate.resources[0].Read(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != string(req.Input) {
		t.Fatalf("expected echoed input %s, got %s", string(req.Input), string(data))
	}
	if delegate.resources[1].Name() != attachment.Name() {
		t.Fatalf("expected attachment %q, got %q", attachment.Name(), delegate.resources[1].Name())
	}
}

func TestCallAgentNotFound(t *testing.T) {
	m := newListAgentsManager(t)

	if _, err := m.CallAgent(context.Background(), "builtin.missing", schema.CallAgentRequest{}, nil); err == nil {
		t.Fatal("expected not found error, got nil")
	}
}

func TestListAgentsWithNamespaceFilter(t *testing.T) {
	m := newListAgentsManager(t)

	resp, err := m.ListAgents(context.Background(), schema.AgentListRequest{Namespace: "missing"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Count != 0 {
		t.Fatalf("expected Count=0, got %d", resp.Count)
	}
	if len(resp.Body) != 0 {
		t.Fatalf("expected 0 agents after namespace filtering, got %d", len(resp.Body))
	}
}

func TestListAgentsWithUserScopedNamespaces(t *testing.T) {
	conn, m := newIntegrationManager(t)
	ctx := llmtest.Context(t)

	if err := m.Exec(ctx, `TRUNCATE llm.connector CASCADE`); err != nil {
		t.Fatal(err)
	}
	if err := m.Toolkit.AddPrompt(&listAgentsMockPrompt{name: "local", description: "builtin"}); err != nil {
		t.Fatal(err)
	}

	publicNamespace := "publicagents"
	privateNamespace := "privateagents"
	publicURL := promptConnectorURL(t, "list-agents-public", &listAgentsMockPrompt{name: "remote_agent", title: "Remote Agent"})
	privateURL := promptConnectorURL(t, "list-agents-private", &listAgentsMockPrompt{name: "remote_agent", title: "Remote Agent"})
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
		resp, err := m.Toolkit.List(ctx, toolkit.ListRequest{Type: toolkit.ListTypePrompts})
		if err != nil {
			return false
		}
		return resp.Count >= 3
	}, "timed out waiting for connector prompts to appear in toolkit")

	adminResp, err := m.ListAgents(ctx, schema.AgentListRequest{}, llmtest.AdminUser(conn))
	if err != nil {
		t.Fatal(err)
	}
	if adminResp.Count != 3 {
		t.Fatalf("expected admin to see 3 agents, got %d", adminResp.Count)
	}

	userResp, err := m.ListAgents(ctx, schema.AgentListRequest{}, llmtest.User(conn))
	if err != nil {
		t.Fatal(err)
	}
	if userResp.Count != 2 {
		t.Fatalf("expected ungrouped user to see 2 agents, got %d", userResp.Count)
	}
	for _, agent := range userResp.Body {
		if agent.Name == privateNamespace+".remote_agent" {
			t.Fatal("expected private agent to be filtered out for ungrouped user")
		}
	}

	privateResp, err := m.ListAgents(ctx, schema.AgentListRequest{Namespace: privateNamespace}, llmtest.User(conn))
	if err != nil {
		t.Fatal(err)
	}
	if privateResp.Count != 0 || len(privateResp.Body) != 0 {
		t.Fatalf("expected inaccessible private namespace to return no agents, got count=%d len=%d", privateResp.Count, len(privateResp.Body))
	}
}

func TestGetAgentWithUserScopedNamespaces(t *testing.T) {
	conn, m := newIntegrationManager(t)
	ctx := llmtest.Context(t)

	if err := m.Exec(ctx, `TRUNCATE llm.connector CASCADE`); err != nil {
		t.Fatal(err)
	}

	publicNamespace := "publicagentget"
	privateNamespace := "privateagentget"
	publicURL := promptConnectorURL(t, "get-agent-public", &listAgentsMockPrompt{name: "remote_agent", title: "Remote Agent"})
	privateURL := promptConnectorURL(t, "get-agent-private", &listAgentsMockPrompt{name: "remote_agent", title: "Remote Agent"})
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
		resp, err := m.Toolkit.List(ctx, toolkit.ListRequest{Type: toolkit.ListTypePrompts})
		if err != nil {
			return false
		}
		return resp.Count >= 2
	}, "timed out waiting for connector prompts to appear in toolkit")

	if _, err := m.GetAgent(ctx, publicNamespace+".remote_agent", llmtest.User(conn)); err != nil {
		t.Fatalf("expected public connector agent lookup to succeed: %v", err)
	}
	if _, err := m.GetAgent(ctx, privateNamespace+".remote_agent", llmtest.User(conn)); err == nil {
		t.Fatal("expected private connector agent lookup to be denied")
	}
	if _, err := m.GetAgent(ctx, privateNamespace+".remote_agent", llmtest.AdminUser(conn)); err != nil {
		t.Fatalf("expected admin private connector agent lookup to succeed: %v", err)
	}
	if _, err := m.GetAgent(ctx, "remote_agent", llmtest.AdminUser(conn)); err == nil {
		t.Fatal("expected bare remote agent lookup to be ambiguous for admin")
	}
}

func TestCallAgentWithUserScopedNamespaces(t *testing.T) {
	conn, m := newIntegrationManager(t)
	ctx := llmtest.Context(t)

	if err := m.Exec(ctx, `TRUNCATE llm.connector CASCADE`); err != nil {
		t.Fatal(err)
	}

	publicNamespace := "publicagentcall"
	privateNamespace := "privateagentcall"
	publicURL := promptConnectorURL(t, "call-agent-public", &listAgentsMockPrompt{name: "remote_agent", title: "Remote Agent"})
	privateURL := promptConnectorURL(t, "call-agent-private", &listAgentsMockPrompt{name: "remote_agent", title: "Remote Agent"})
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
		resp, err := m.Toolkit.List(ctx, toolkit.ListRequest{Type: toolkit.ListTypePrompts})
		if err != nil {
			return false
		}
		return resp.Count >= 2
	}, "timed out waiting for connector prompts to appear in toolkit")

	if _, err := m.CallAgent(ctx, publicNamespace+".remote_agent", schema.CallAgentRequest{}, llmtest.User(conn)); err == nil {
		t.Fatal("expected current prompt execution to fail for public agent")
	}
	if _, err := m.CallAgent(ctx, privateNamespace+".remote_agent", schema.CallAgentRequest{}, llmtest.User(conn)); err == nil {
		t.Fatal("expected private connector agent call to be denied")
	}
	if _, err := m.CallAgent(ctx, privateNamespace+".remote_agent", schema.CallAgentRequest{}, llmtest.AdminUser(conn)); err == nil {
		t.Fatal("expected current prompt execution to fail for private agent")
	}
	if _, err := m.CallAgent(ctx, "remote_agent", schema.CallAgentRequest{}, llmtest.AdminUser(conn)); err == nil {
		t.Fatal("expected bare remote agent call to be ambiguous for admin")
	}
}

func newListAgentsManager(t *testing.T) *Manager {
	tk, err := toolkit.New()
	if err != nil {
		t.Fatal(err)
	}
	if err := tk.AddPrompt(
		&listAgentsMockPrompt{name: "charlie", title: "Charlie Agent", description: "C"},
		&listAgentsMockPrompt{name: "alpha", title: "Alpha Agent", description: "A"},
		&listAgentsMockPrompt{name: "bravo", title: "Bravo Agent", description: "B"},
	); err != nil {
		t.Fatal(err)
	}

	return &Manager{Toolkit: tk}
}

func promptConnectorURL(t *testing.T, name string, prompts ...llm.Prompt) string {
	t.Helper()

	srv, err := mcpserver.New(name, "1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	srv.AddPrompts(prompts...)

	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(func() {
		ts.CloseClientConnections()
		ts.Close()
	})

	return ts.URL
}
