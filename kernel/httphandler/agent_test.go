package httphandler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	llmmanager "github.com/mutablelogic/go-llm/kernel/manager"
	opt "github.com/mutablelogic/go-llm/pkg/opt"
	schema "github.com/mutablelogic/go-llm/kernel/schema"
	toolkit "github.com/mutablelogic/go-llm/toolkit"
	resource "github.com/mutablelogic/go-llm/toolkit/resource"
	types "github.com/mutablelogic/go-server/pkg/types"
)

type agentHandlerMockPrompt struct {
	name        string
	title       string
	description string
}

func (p *agentHandlerMockPrompt) Name() string        { return p.name }
func (p *agentHandlerMockPrompt) Title() string       { return p.title }
func (p *agentHandlerMockPrompt) Description() string { return p.description }
func (p *agentHandlerMockPrompt) Prepare(context.Context, json.RawMessage) (string, []opt.Opt, error) {
	return "", nil, nil
}

type agentHandlerMockDelegate struct {
	call func(context.Context, llm.Prompt, ...llm.Resource) (llm.Resource, error)
}

func (d *agentHandlerMockDelegate) OnEvent(toolkit.ConnectorEvent) {}

func (d *agentHandlerMockDelegate) Call(ctx context.Context, prompt llm.Prompt, resources ...llm.Resource) (llm.Resource, error) {
	if d.call != nil {
		return d.call(ctx, prompt, resources...)
	}
	return nil, nil
}

func (d *agentHandlerMockDelegate) CreateConnector(string, func(toolkit.ConnectorEvent)) (llm.Connector, error) {
	return nil, nil
}

func TestAgentList(t *testing.T) {
	manager := &llmmanager.Manager{Toolkit: mustAgentToolkit(t)}
	_, _, item := AgentHandler(manager)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/agent?limit=2", nil)
	item.Handler().ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp schema.AgentList
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Count != 3 {
		t.Fatalf("expected count=3, got %d", resp.Count)
	}
	if len(resp.Body) != 2 {
		t.Fatalf("expected 2 agents, got %d", len(resp.Body))
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

func TestAgentListInvalidQuery(t *testing.T) {
	_, _, item := AgentHandler(&llmmanager.Manager{})

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/agent?limit=invalid", nil)
	item.Handler().ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAgentListWithFilters(t *testing.T) {
	manager := &llmmanager.Manager{Toolkit: mustAgentToolkit(t)}
	_, _, item := AgentHandler(manager)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/agent?namespace=builtin&name=builtin.bravo&name=builtin.alpha", nil)
	item.Handler().ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp schema.AgentList
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Count != 2 || len(resp.Body) != 2 {
		t.Fatalf("expected 2 filtered agents, got count=%d len=%d", resp.Count, len(resp.Body))
	}
	if resp.Body[0].Name != "builtin.alpha" || resp.Body[1].Name != "builtin.bravo" {
		t.Fatalf("unexpected filtered agents: %+v", resp.Body)
	}
}

func TestGetAgent(t *testing.T) {
	manager := &llmmanager.Manager{Toolkit: mustAgentToolkit(t)}
	_, _, item := AgentResourceHandler(manager)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/agent/builtin.alpha", nil)
	r.SetPathValue("name", "builtin.alpha")
	item.Handler().ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp schema.AgentMeta
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Name != "builtin.alpha" {
		t.Fatalf("expected agent %q, got %q", "builtin.alpha", resp.Name)
	}
	if resp.Title != "Alpha Agent" {
		t.Fatalf("expected title %q, got %q", "Alpha Agent", resp.Title)
	}
}

func TestGetAgentUnescapesName(t *testing.T) {
	manager := &llmmanager.Manager{Toolkit: mustAgentToolkit(t)}
	_, _, item := AgentResourceHandler(manager)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/agent/builtin%2Ealpha", nil)
	r.SetPathValue("name", "builtin%2Ealpha")
	item.Handler().ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp schema.AgentMeta
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Name != "builtin.alpha" {
		t.Fatalf("expected unescaped agent name %q, got %q", "builtin.alpha", resp.Name)
	}
}

func TestGetAgentNotFound(t *testing.T) {
	manager := &llmmanager.Manager{Toolkit: mustAgentToolkit(t)}
	_, _, item := AgentResourceHandler(manager)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/agent/builtin.missing", nil)
	r.SetPathValue("name", "builtin.missing")
	item.Handler().ServeHTTP(w, r)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCallAgent(t *testing.T) {
	manager := &llmmanager.Manager{Toolkit: mustAgentToolkitWithDelegate(t, &agentHandlerMockDelegate{
		call: func(_ context.Context, prompt llm.Prompt, resources ...llm.Resource) (llm.Resource, error) {
			if prompt.Name() != "builtin.alpha" {
				t.Fatalf("expected prompt %q, got %q", "builtin.alpha", prompt.Name())
			}
			if len(resources) != 1 {
				t.Fatalf("expected 1 resource, got %d", len(resources))
			}
			body, err := resources[0].Read(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if string(body) != `{"query":"docs"}` {
				t.Fatalf("unexpected resource body: %s", string(body))
			}
			result, err := resource.Text("answer", "resolved")
			if err != nil {
				t.Fatal(err)
			}
			return result, nil
		},
	})}
	_, _, item := AgentResourceHandler(manager)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/agent/builtin.alpha", bytes.NewReader([]byte(`{"input":{"query":"docs"}}`)))
	r.Header.Set(types.ContentTypeHeader, types.ContentTypeJSON)
	r.SetPathValue("name", "builtin.alpha")
	item.Handler().ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if got := w.Header().Get(types.ContentTypeHeader); got != types.ContentTypeTextPlain {
		t.Fatalf("expected content type %q, got %q", types.ContentTypeTextPlain, got)
	}
	if got := w.Header().Get(types.ContentPathHeader); got != "text:answer" {
		t.Fatalf("expected content path %q, got %q", "text:answer", got)
	}
	if got := w.Header().Get(types.ContentNameHeader); got != "answer" {
		t.Fatalf("expected content name %q, got %q", "answer", got)
	}
	if got := w.Body.String(); got != "resolved" {
		t.Fatalf("unexpected response body: %s", got)
	}
}

func TestCallAgentInvalidBody(t *testing.T) {
	manager := &llmmanager.Manager{Toolkit: mustAgentToolkit(t)}
	_, _, item := AgentResourceHandler(manager)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/agent/builtin.alpha", bytes.NewReader([]byte(`{"input":`)))
	r.Header.Set(types.ContentTypeHeader, types.ContentTypeJSON)
	r.SetPathValue("name", "builtin.alpha")
	item.Handler().ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCallAgentUnescapesName(t *testing.T) {
	manager := &llmmanager.Manager{Toolkit: mustAgentToolkitWithDelegate(t, &agentHandlerMockDelegate{
		call: func(_ context.Context, prompt llm.Prompt, resources ...llm.Resource) (llm.Resource, error) {
			if prompt.Name() != "builtin.alpha" {
				t.Fatalf("expected prompt %q, got %q", "builtin.alpha", prompt.Name())
			}
			result, err := resource.Text("answer", "resolved")
			if err != nil {
				t.Fatal(err)
			}
			return result, nil
		},
	})}
	_, _, item := AgentResourceHandler(manager)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/agent/builtin%2Ealpha", bytes.NewReader([]byte(`{"input":{"query":"docs"}}`)))
	r.Header.Set(types.ContentTypeHeader, types.ContentTypeJSON)
	r.SetPathValue("name", "builtin%2Ealpha")
	item.Handler().ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if got := w.Body.String(); got != "resolved" {
		t.Fatalf("unexpected response body: %s", got)
	}
}

func mustAgentToolkit(t *testing.T) toolkit.Toolkit {
	t.Helper()
	return mustAgentToolkitWithDelegate(t, nil)
}

func mustAgentToolkitWithDelegate(t *testing.T, delegate toolkit.ToolkitDelegate) toolkit.Toolkit {
	t.Helper()
	var tk toolkit.Toolkit
	var err error
	if delegate != nil {
		tk, err = toolkit.New(toolkit.WithDelegate(delegate))
	} else {
		tk, err = toolkit.New()
	}
	if err != nil {
		t.Fatal(err)
	}
	if err := tk.AddPrompt(
		&agentHandlerMockPrompt{name: "alpha", title: "Alpha Agent", description: "A"},
		&agentHandlerMockPrompt{name: "bravo", title: "Bravo Agent", description: "B"},
		&agentHandlerMockPrompt{name: "charlie", title: "Charlie Agent", description: "C"},
	); err != nil {
		t.Fatal(err)
	}

	return tk
}
