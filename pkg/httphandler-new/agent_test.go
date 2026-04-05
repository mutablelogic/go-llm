package httphandler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	// Packages
	llmmanager "github.com/mutablelogic/go-llm/pkg/llmmanager"
	opt "github.com/mutablelogic/go-llm/pkg/opt"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	toolkit "github.com/mutablelogic/go-llm/pkg/toolkit"
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

func mustAgentToolkit(t *testing.T) toolkit.Toolkit {
	t.Helper()
	tk, err := toolkit.New()
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
