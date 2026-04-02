package httphandler_test

import (
	"bufio"
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	// Packages
	schema "github.com/mutablelogic/go-llm/pkg/schema"
)

///////////////////////////////////////////////////////////////////////////////
// MODEL LIST TESTS

func TestModelList_OK(t *testing.T) {
	mgr := newTestManager(t, []mockClient{
		{name: "provider-1", models: []schema.Model{{Name: "model-a"}, {Name: "model-b"}}},
	})
	mux := serveMux(mgr)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/model", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp schema.ListModelsResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Count != 2 {
		t.Fatalf("expected count=2, got %d", resp.Count)
	}
	if len(resp.Body) != 2 {
		t.Fatalf("expected 2 models, got %d", len(resp.Body))
	}
}

func TestModelList_WithPagination(t *testing.T) {
	mgr := newTestManager(t, []mockClient{
		{name: "provider-1", models: []schema.Model{{Name: "a"}, {Name: "b"}, {Name: "c"}}},
	})
	mux := serveMux(mgr)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/model?limit=2&offset=1", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp schema.ListModelsResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Count != 3 {
		t.Fatalf("expected count=3, got %d", resp.Count)
	}
	if len(resp.Body) != 2 {
		t.Fatalf("expected 2 models in page, got %d", len(resp.Body))
	}
	if resp.Body[0].Name != "b" {
		t.Fatalf("expected first model=b, got %q", resp.Body[0].Name)
	}
}

func TestModelList_MethodNotAllowed(t *testing.T) {
	mgr := newTestManager(t, []mockClient{
		{name: "provider-1"},
	})
	mux := serveMux(mgr)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPut, "/model", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

///////////////////////////////////////////////////////////////////////////////
// MODEL GET TESTS

func TestModelGet_OK(t *testing.T) {
	mgr := newTestManager(t, []mockClient{
		{name: "provider-1", models: []schema.Model{{Name: "gpt-4", OwnedBy: "provider-1"}}},
	})
	mux := serveMux(mgr)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/model/gpt-4", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var model schema.Model
	if err := json.NewDecoder(w.Body).Decode(&model); err != nil {
		t.Fatal(err)
	}
	if model.Name != "gpt-4" {
		t.Fatalf("expected name=gpt-4, got %q", model.Name)
	}
}

func TestModelGet_WithProvider(t *testing.T) {
	mgr := newTestManager(t, []mockClient{
		{name: "provider-1", models: []schema.Model{{Name: "model-x"}}},
		{name: "provider-2", models: []schema.Model{{Name: "model-x"}}},
	})
	mux := serveMux(mgr)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/model/provider-1/model-x", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var model schema.Model
	if err := json.NewDecoder(w.Body).Decode(&model); err != nil {
		t.Fatal(err)
	}
	if model.Name != "model-x" {
		t.Fatalf("expected name=model-x, got %q", model.Name)
	}
}

func TestModelGet_NotFound(t *testing.T) {
	mgr := newTestManager(t, []mockClient{
		{name: "provider-1", models: []schema.Model{{Name: "model-a"}}},
	})
	mux := serveMux(mgr)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/model/nonexistent", nil)
	mux.ServeHTTP(w, r)

	// schema.ErrNotFound maps to 404 via httpErr
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

///////////////////////////////////////////////////////////////////////////////
// MODEL DOWNLOAD TESTS

func TestModelDownload_JSON(t *testing.T) {
	mgr := newTestManagerWithDownloader(t, []*mockDownloaderClient{
		{mockClient: mockClient{name: "provider-1"}},
	})
	mux := serveMux(mgr)

	body, _ := json.Marshal(schema.DownloadModelRequest{Name: "llama3"})
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/model", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var model schema.Model
	if err := json.NewDecoder(w.Body).Decode(&model); err != nil {
		t.Fatal(err)
	}
	if model.Name != "llama3" {
		t.Fatalf("expected name=llama3, got %q", model.Name)
	}
}

func TestModelDownload_Stream(t *testing.T) {
	mgr := newTestManagerWithDownloader(t, []*mockDownloaderClient{
		{mockClient: mockClient{name: "provider-1"}},
	})
	mux := serveMux(mgr)

	body, _ := json.Marshal(schema.DownloadModelRequest{Name: "llama3"})
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/model", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Accept", "text/event-stream")
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); ct != "text/event-stream" {
		t.Fatalf("expected Content-Type text/event-stream, got %q", ct)
	}

	type sseEvent struct{ name, data string }
	var events []sseEvent
	scanner := bufio.NewScanner(w.Body)
	var cur sseEvent
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case strings.HasPrefix(line, "event: "):
			cur.name = strings.TrimPrefix(line, "event: ")
		case strings.HasPrefix(line, "data: "):
			cur.data = strings.TrimPrefix(line, "data: ")
		case line == "":
			if cur.name != "" || cur.data != "" {
				events = append(events, cur)
				cur = sseEvent{}
			}
		}
	}

	var foundResult bool
	for _, evt := range events {
		if evt.name == schema.EventResult {
			foundResult = true
			var model schema.Model
			if err := json.Unmarshal([]byte(evt.data), &model); err != nil {
				t.Fatalf("failed to decode result event: %v", err)
			}
			if model.Name != "llama3" {
				t.Fatalf("expected name=llama3, got %q", model.Name)
			}
		}
	}
	if !foundResult {
		t.Fatalf("no 'result' event found in SSE stream; events: %+v", events)
	}
}

func TestModelDownload_NotAcceptable(t *testing.T) {
	mgr := newTestManagerWithDownloader(t, []*mockDownloaderClient{
		{mockClient: mockClient{name: "provider-1"}},
	})
	mux := serveMux(mgr)

	body, _ := json.Marshal(schema.DownloadModelRequest{Name: "llama3"})
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/model", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Accept", "text/plain")
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusNotAcceptable {
		t.Fatalf("expected 406, got %d", w.Code)
	}
}

///////////////////////////////////////////////////////////////////////////////
// MODEL DELETE TESTS

func TestModelDelete_OK(t *testing.T) {
	mgr := newTestManagerWithDownloader(t, []*mockDownloaderClient{
		{mockClient: mockClient{name: "provider-1", models: []schema.Model{{Name: "llama3"}}}},
	})
	mux := serveMux(mgr)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodDelete, "/model/llama3", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", w.Code, w.Body.String())
	}
}

func TestModelDelete_WithProvider(t *testing.T) {
	mgr := newTestManagerWithDownloader(t, []*mockDownloaderClient{
		{mockClient: mockClient{name: "provider-1", models: []schema.Model{{Name: "llama3"}}}},
	})
	mux := serveMux(mgr)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodDelete, "/model/provider-1/llama3", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", w.Code, w.Body.String())
	}
}

func TestModelDelete_NotFound(t *testing.T) {
	mgr := newTestManagerWithDownloader(t, []*mockDownloaderClient{
		{mockClient: mockClient{name: "provider-1", models: []schema.Model{{Name: "llama3"}}}},
	})
	mux := serveMux(mgr)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodDelete, "/model/nonexistent", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}
