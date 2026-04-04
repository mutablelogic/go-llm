package httphandler_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	schema "github.com/mutablelogic/go-llm/pkg/schema"
)

///////////////////////////////////////////////////////////////////////////////
// AGENT LIST TESTS

func TestAgentList_OK(t *testing.T) {
	mgr := newTestManager(t, []mockClient{
		{name: "provider-1", models: []schema.Model{{Name: "model-a"}}},
	})
	mux := serveMux(mgr)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/agent", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp schema.ListAgentResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Count != 0 {
		t.Fatalf("expected count=0, got %d", resp.Count)
	}
}

func TestAgentList_WithAgents(t *testing.T) {
	mgr := newTestManager(t, []mockClient{
		{name: "provider-1", models: []schema.Model{{Name: "model-a"}}},
	})
	mux := serveMux(mgr)

	for _, name := range []string{"agent-one", "agent-two"} {
		body, _ := json.Marshal(schema.AgentMeta{Name: name})
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/agent", bytes.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		mux.ServeHTTP(w, r)
		if w.Code != http.StatusCreated {
			t.Fatalf("create %s: expected 201, got %d: %s", name, w.Code, w.Body.String())
		}
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/agent", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp schema.ListAgentResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Count != 2 {
		t.Fatalf("expected count=2, got %d", resp.Count)
	}
	if len(resp.Body) != 2 {
		t.Fatalf("expected 2 agents, got %d", len(resp.Body))
	}
}

func TestAgentList_WithPagination(t *testing.T) {
	mgr := newTestManager(t, []mockClient{
		{name: "provider-1", models: []schema.Model{{Name: "model-a"}}},
	})
	mux := serveMux(mgr)

	for _, name := range []string{"agent-a", "agent-b", "agent-c"} {
		body, _ := json.Marshal(schema.AgentMeta{Name: name})
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/agent", bytes.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		mux.ServeHTTP(w, r)
		if w.Code != http.StatusCreated {
			t.Fatalf("create %s: expected 201, got %d: %s", name, w.Code, w.Body.String())
		}
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/agent?limit=2&offset=1", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp schema.ListAgentResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Count != 3 {
		t.Fatalf("expected count=3, got %d", resp.Count)
	}
	if len(resp.Body) != 2 {
		t.Fatalf("expected 2 agents in page, got %d", len(resp.Body))
	}
}

func TestAgentList_FilterByName(t *testing.T) {
	mgr := newTestManager(t, []mockClient{
		{name: "provider-1", models: []schema.Model{{Name: "model-a"}}},
	})
	mux := serveMux(mgr)

	for _, name := range []string{"alpha", "beta"} {
		body, _ := json.Marshal(schema.AgentMeta{Name: name})
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/agent", bytes.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		mux.ServeHTTP(w, r)
		if w.Code != http.StatusCreated {
			t.Fatalf("create %s: expected 201, got %d: %s", name, w.Code, w.Body.String())
		}
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/agent?name=alpha", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp schema.ListAgentResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Count != 1 {
		t.Fatalf("expected count=1, got %d", resp.Count)
	}
	if len(resp.Body) != 1 {
		t.Fatalf("expected 1 agent, got %d", len(resp.Body))
	}
	if resp.Body[0].Name != "alpha" {
		t.Fatalf("expected name=alpha, got %q", resp.Body[0].Name)
	}
}

///////////////////////////////////////////////////////////////////////////////
// AGENT CREATE TESTS

func TestAgentCreate_OK(t *testing.T) {
	mgr := newTestManager(t, []mockClient{
		{name: "provider-1", models: []schema.Model{{Name: "gpt-4"}}},
	})
	mux := serveMux(mgr)

	body, _ := json.Marshal(schema.AgentMeta{
		Name:  "my-agent",
		Title: "My Agent",
	})
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/agent", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var agent schema.Agent
	if err := json.NewDecoder(w.Body).Decode(&agent); err != nil {
		t.Fatal(err)
	}
	if agent.ID == "" {
		t.Fatal("expected non-empty agent ID")
	}
	if agent.Name != "my-agent" {
		t.Fatalf("expected name=my-agent, got %q", agent.Name)
	}
	if agent.Title != "My Agent" {
		t.Fatalf("expected title='My Agent', got %q", agent.Title)
	}
	if agent.Version != 1 {
		t.Fatalf("expected version=1, got %d", agent.Version)
	}
}

func TestAgentCreate_WithModel(t *testing.T) {
	mgr := newTestManager(t, []mockClient{
		{name: "provider-1", models: []schema.Model{{Name: "gpt-4"}}},
	})
	mux := serveMux(mgr)

	body, _ := json.Marshal(schema.AgentMeta{
		Name:          "model-agent",
		GeneratorMeta: schema.GeneratorMeta{Model: "gpt-4"},
	})
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/agent", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var agent schema.Agent
	if err := json.NewDecoder(w.Body).Decode(&agent); err != nil {
		t.Fatal(err)
	}
	if agent.Model != "gpt-4" {
		t.Fatalf("expected model=gpt-4, got %q", agent.Model)
	}
	if agent.Provider != "provider-1" {
		t.Fatalf("expected provider=provider-1, got %q", agent.Provider)
	}
}

func TestAgentCreate_DuplicateName(t *testing.T) {
	mgr := newTestManager(t, []mockClient{
		{name: "provider-1", models: []schema.Model{{Name: "model-a"}}},
	})
	mux := serveMux(mgr)

	body, _ := json.Marshal(schema.AgentMeta{Name: "duplicate"})
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/agent", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusCreated {
		t.Fatalf("create: expected 201, got %d: %s", w.Code, w.Body.String())
	}

	body, _ = json.Marshal(schema.AgentMeta{Name: "duplicate"})
	w = httptest.NewRecorder()
	r = httptest.NewRequest(http.MethodPost, "/agent", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAgentCreate_InvalidJSON(t *testing.T) {
	mgr := newTestManager(t, []mockClient{
		{name: "provider-1", models: []schema.Model{{Name: "model-a"}}},
	})
	mux := serveMux(mgr)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/agent", bytes.NewBufferString("{invalid"))
	r.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAgent_MethodNotAllowed(t *testing.T) {
	mgr := newTestManager(t, []mockClient{
		{name: "provider-1", models: []schema.Model{{Name: "model-a"}}},
	})
	mux := serveMux(mgr)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodDelete, "/agent", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

///////////////////////////////////////////////////////////////////////////////
// AGENT GET TESTS

func TestAgentGet_ByID(t *testing.T) {
	mgr := newTestManager(t, []mockClient{
		{name: "provider-1", models: []schema.Model{{Name: "model-a"}}},
	})
	mux := serveMux(mgr)

	body, _ := json.Marshal(schema.AgentMeta{Name: "get-test"})
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/agent", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusCreated {
		t.Fatalf("create: expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var created schema.Agent
	if err := json.NewDecoder(w.Body).Decode(&created); err != nil {
		t.Fatal(err)
	}

	w = httptest.NewRecorder()
	r = httptest.NewRequest(http.MethodGet, "/agent/"+created.ID, nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var agent schema.Agent
	if err := json.NewDecoder(w.Body).Decode(&agent); err != nil {
		t.Fatal(err)
	}
	if agent.ID != created.ID {
		t.Fatalf("expected id=%s, got %s", created.ID, agent.ID)
	}
	if agent.Name != "get-test" {
		t.Fatalf("expected name=get-test, got %q", agent.Name)
	}
}

func TestAgentGet_ByName(t *testing.T) {
	mgr := newTestManager(t, []mockClient{
		{name: "provider-1", models: []schema.Model{{Name: "model-a"}}},
	})
	mux := serveMux(mgr)

	body, _ := json.Marshal(schema.AgentMeta{Name: "named-agent"})
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/agent", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusCreated {
		t.Fatalf("create: expected 201, got %d: %s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	r = httptest.NewRequest(http.MethodGet, "/agent/named-agent", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var agent schema.Agent
	if err := json.NewDecoder(w.Body).Decode(&agent); err != nil {
		t.Fatal(err)
	}
	if agent.Name != "named-agent" {
		t.Fatalf("expected name=named-agent, got %q", agent.Name)
	}
}

func TestAgentGet_NotFound(t *testing.T) {
	mgr := newTestManager(t, []mockClient{
		{name: "provider-1", models: []schema.Model{{Name: "model-a"}}},
	})
	mux := serveMux(mgr)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/agent/nonexistent", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

///////////////////////////////////////////////////////////////////////////////
// AGENT DELETE TESTS

func TestAgentDelete_OK(t *testing.T) {
	mgr := newTestManager(t, []mockClient{
		{name: "provider-1", models: []schema.Model{{Name: "model-a"}}},
	})
	mux := serveMux(mgr)

	body, _ := json.Marshal(schema.AgentMeta{Name: "doomed"})
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/agent", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusCreated {
		t.Fatalf("create: expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var created schema.Agent
	if err := json.NewDecoder(w.Body).Decode(&created); err != nil {
		t.Fatal(err)
	}

	w = httptest.NewRecorder()
	r = httptest.NewRequest(http.MethodDelete, "/agent/"+created.ID, nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", w.Code, w.Body.String())
	}
	if w.Body.Len() != 0 {
		t.Fatalf("expected empty body, got %s", w.Body.String())
	}

	w = httptest.NewRecorder()
	r = httptest.NewRequest(http.MethodGet, "/agent/"+created.ID, nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 after delete, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAgentDelete_NotFound(t *testing.T) {
	mgr := newTestManager(t, []mockClient{
		{name: "provider-1", models: []schema.Model{{Name: "model-a"}}},
	})
	mux := serveMux(mgr)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodDelete, "/agent/nonexistent", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAgentGet_MethodNotAllowed(t *testing.T) {
	mgr := newTestManager(t, []mockClient{
		{name: "provider-1", models: []schema.Model{{Name: "model-a"}}},
	})
	mux := serveMux(mgr)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPatch, "/agent/some-id", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

///////////////////////////////////////////////////////////////////////////////
// AGENT PUT TESTS

func TestAgentPut_OK(t *testing.T) {
	mgr := newTestManager(t, []mockClient{
		{name: "provider-1", models: []schema.Model{{Name: "gpt-4"}}},
	})
	mux := serveMux(mgr)

	body, _ := json.Marshal(schema.AgentMeta{Name: "original", Title: "Original Title"})
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/agent", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusCreated {
		t.Fatalf("create: expected 201, got %d: %s", w.Code, w.Body.String())
	}

	body, _ = json.Marshal(schema.AgentMeta{Name: "original", Title: "Updated Title"})
	w = httptest.NewRecorder()
	r = httptest.NewRequest(http.MethodPut, "/agent", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var updated schema.Agent
	if err := json.NewDecoder(w.Body).Decode(&updated); err != nil {
		t.Fatal(err)
	}
	if updated.Title != "Updated Title" {
		t.Fatalf("expected title='Updated Title', got %q", updated.Title)
	}
	if updated.Version != 2 {
		t.Fatalf("expected version=2, got %d", updated.Version)
	}
}

func TestAgentPut_NotModified(t *testing.T) {
	mgr := newTestManager(t, []mockClient{
		{name: "provider-1", models: []schema.Model{{Name: "gpt-4"}}},
	})
	mux := serveMux(mgr)

	body, _ := json.Marshal(schema.AgentMeta{Name: "unchanged", Title: "Same Title Here"})
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/agent", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusCreated {
		t.Fatalf("create: expected 201, got %d: %s", w.Code, w.Body.String())
	}

	// PUT with identical data
	body, _ = json.Marshal(schema.AgentMeta{Name: "unchanged", Title: "Same Title Here"})
	w = httptest.NewRecorder()
	r = httptest.NewRequest(http.MethodPut, "/agent", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusNotModified {
		t.Fatalf("expected 304, got %d: %s", w.Code, w.Body.String())
	}
	if w.Body.Len() != 0 {
		t.Fatalf("expected empty body for 304, got %s", w.Body.String())
	}
}

func TestAgentPut_WithModel(t *testing.T) {
	mgr := newTestManager(t, []mockClient{
		{name: "provider-1", models: []schema.Model{{Name: "gpt-4"}}},
	})
	mux := serveMux(mgr)

	body, _ := json.Marshal(schema.AgentMeta{Name: "no-model"})
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/agent", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusCreated {
		t.Fatalf("create: expected 201, got %d: %s", w.Code, w.Body.String())
	}

	body, _ = json.Marshal(schema.AgentMeta{Name: "no-model", GeneratorMeta: schema.GeneratorMeta{Model: "gpt-4"}})
	w = httptest.NewRecorder()
	r = httptest.NewRequest(http.MethodPut, "/agent", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var updated schema.Agent
	if err := json.NewDecoder(w.Body).Decode(&updated); err != nil {
		t.Fatal(err)
	}
	if updated.Model != "gpt-4" {
		t.Fatalf("expected model=gpt-4, got %q", updated.Model)
	}
	if updated.Provider != "provider-1" {
		t.Fatalf("expected provider=provider-1, got %q", updated.Provider)
	}
}

func TestAgentPut_NotFound(t *testing.T) {
	mgr := newTestManager(t, []mockClient{
		{name: "provider-1", models: []schema.Model{{Name: "model-a"}}},
	})
	mux := serveMux(mgr)

	body, _ := json.Marshal(schema.AgentMeta{Name: "nonexistent", Title: "Does Not Exist"})
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPut, "/agent", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAgentPut_InvalidJSON(t *testing.T) {
	mgr := newTestManager(t, []mockClient{
		{name: "provider-1", models: []schema.Model{{Name: "model-a"}}},
	})
	mux := serveMux(mgr)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPut, "/agent", bytes.NewBufferString("{invalid"))
	r.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

///////////////////////////////////////////////////////////////////////////////
// AGENT MARKDOWN/TEXT CONTENT TYPE TESTS

func TestAgentCreate_Markdown(t *testing.T) {
	mgr := newTestManager(t, []mockClient{
		{name: "provider-1", models: []schema.Model{{Name: "gpt-4"}}},
	})
	mux := serveMux(mgr)

	md := "---\nname: md-agent\ntitle: Markdown Agent\nmodel: gpt-4\n---\nYou are a helpful assistant.\n"
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/agent", bytes.NewBufferString(md))
	r.Header.Set("Content-Type", "text/markdown")
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var agent schema.Agent
	if err := json.NewDecoder(w.Body).Decode(&agent); err != nil {
		t.Fatal(err)
	}
	if agent.Name != "md-agent" {
		t.Fatalf("expected name=md-agent, got %q", agent.Name)
	}
	if agent.Title != "Markdown Agent" {
		t.Fatalf("expected title='Markdown Agent', got %q", agent.Title)
	}
	if agent.Model != "gpt-4" {
		t.Fatalf("expected model=gpt-4, got %q", agent.Model)
	}
	if agent.Template != "You are a helpful assistant." {
		t.Fatalf("expected template='You are a helpful assistant.', got %q", agent.Template)
	}
}

func TestAgentCreate_TextPlain(t *testing.T) {
	mgr := newTestManager(t, []mockClient{
		{name: "provider-1", models: []schema.Model{{Name: "model-a"}}},
	})
	mux := serveMux(mgr)

	md := "---\nname: text-agent\ntitle: Text Plain Agent\n---\nPlain text template.\n"
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/agent", bytes.NewBufferString(md))
	r.Header.Set("Content-Type", "text/plain")
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var agent schema.Agent
	if err := json.NewDecoder(w.Body).Decode(&agent); err != nil {
		t.Fatal(err)
	}
	if agent.Name != "text-agent" {
		t.Fatalf("expected name=text-agent, got %q", agent.Name)
	}
	if agent.Template != "Plain text template." {
		t.Fatalf("expected template='Plain text template.', got %q", agent.Template)
	}
}

func TestAgentCreate_MarkdownWithCharset(t *testing.T) {
	mgr := newTestManager(t, []mockClient{
		{name: "provider-1", models: []schema.Model{{Name: "model-a"}}},
	})
	mux := serveMux(mgr)

	md := "---\nname: charset-agent\ntitle: Charset Agent Test\n---\nTemplate body.\n"
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/agent", bytes.NewBufferString(md))
	r.Header.Set("Content-Type", "text/markdown; charset=utf-8")
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var agent schema.Agent
	if err := json.NewDecoder(w.Body).Decode(&agent); err != nil {
		t.Fatal(err)
	}
	if agent.Name != "charset-agent" {
		t.Fatalf("expected name=charset-agent, got %q", agent.Name)
	}
}

func TestAgentCreate_MarkdownInvalid(t *testing.T) {
	mgr := newTestManager(t, []mockClient{
		{name: "provider-1", models: []schema.Model{{Name: "model-a"}}},
	})
	mux := serveMux(mgr)

	// Missing closing --- in front matter
	md := "---\nname: bad\ntitle: Bad Agent\n"
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/agent", bytes.NewBufferString(md))
	r.Header.Set("Content-Type", "text/markdown")
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAgentPut_Markdown(t *testing.T) {
	mgr := newTestManager(t, []mockClient{
		{name: "provider-1", models: []schema.Model{{Name: "gpt-4"}}},
	})
	mux := serveMux(mgr)

	// Create via JSON first
	body, _ := json.Marshal(schema.AgentMeta{Name: "put-md", Title: "Put Markdown Test"})
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/agent", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusCreated {
		t.Fatalf("create: expected 201, got %d: %s", w.Code, w.Body.String())
	}

	// PUT via markdown
	md := "---\nname: put-md\ntitle: Updated Markdown Title\nmodel: gpt-4\n---\nNew template body.\n"
	w = httptest.NewRecorder()
	r = httptest.NewRequest(http.MethodPut, "/agent", bytes.NewBufferString(md))
	r.Header.Set("Content-Type", "text/markdown")
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var updated schema.Agent
	if err := json.NewDecoder(w.Body).Decode(&updated); err != nil {
		t.Fatal(err)
	}
	if updated.Title != "Updated Markdown Title" {
		t.Fatalf("expected title='Updated Markdown Title', got %q", updated.Title)
	}
	if updated.Template != "New template body." {
		t.Fatalf("expected template='New template body.', got %q", updated.Template)
	}
	if updated.Version != 2 {
		t.Fatalf("expected version=2, got %d", updated.Version)
	}
}

func TestAgentPut_TextPlain(t *testing.T) {
	mgr := newTestManager(t, []mockClient{
		{name: "provider-1", models: []schema.Model{{Name: "model-a"}}},
	})
	mux := serveMux(mgr)

	// Create via JSON
	body, _ := json.Marshal(schema.AgentMeta{Name: "put-txt", Title: "Put Text Test!!"})
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/agent", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusCreated {
		t.Fatalf("create: expected 201, got %d: %s", w.Code, w.Body.String())
	}

	// PUT via text/plain
	md := "---\nname: put-txt\ntitle: Updated Text Title\n---\nUpdated template.\n"
	w = httptest.NewRecorder()
	r = httptest.NewRequest(http.MethodPut, "/agent", bytes.NewBufferString(md))
	r.Header.Set("Content-Type", "text/plain")
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var updated schema.Agent
	if err := json.NewDecoder(w.Body).Decode(&updated); err != nil {
		t.Fatal(err)
	}
	if updated.Title != "Updated Text Title" {
		t.Fatalf("expected title='Updated Text Title', got %q", updated.Title)
	}
	if updated.Template != "Updated template." {
		t.Fatalf("expected template='Updated template.', got %q", updated.Template)
	}
}
