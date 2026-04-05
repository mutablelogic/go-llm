package httphandler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	// Packages
	llmmanager "github.com/mutablelogic/go-llm/pkg/llmmanager"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	llmtest "github.com/mutablelogic/go-llm/pkg/test"
	types "github.com/mutablelogic/go-server/pkg/types"
)

func TestEmbeddingJSONIntegration(t *testing.T) {
	conn, manager := newModelHandlerIntegrationManager(t)
	modelName := requireEmbeddingModel(t, conn, manager)
	_, _, item := EmbeddingHandler(manager)

	body, err := json.Marshal(schema.EmbeddingRequest{
		Provider: conn.Config.Name,
		Model:    modelName,
		Input:    []string{"hello world"},
	})
	if err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/embedding", bytes.NewReader(body)).WithContext(newModelHandlerTestContext(t))
	r.Header.Set(types.ContentTypeHeader, types.ContentTypeJSON)
	item.Handler().ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp schema.EmbeddingResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Provider != conn.Config.Name {
		t.Fatalf("expected provider %q, got %q", conn.Config.Name, resp.Provider)
	}
	if resp.Model != modelName {
		t.Fatalf("expected model %q, got %q", modelName, resp.Model)
	}
	if len(resp.Output) != 1 {
		t.Fatalf("expected 1 embedding, got %d", len(resp.Output))
	}
	if len(resp.Output[0]) == 0 {
		t.Fatal("expected embedding vector")
	}
	if resp.OutputDimensionality == 0 {
		t.Fatal("expected output dimensionality")
	}
}

func TestEmbeddingBatchIntegration(t *testing.T) {
	conn, manager := newModelHandlerIntegrationManager(t)
	modelName := requireEmbeddingModel(t, conn, manager)
	_, _, item := EmbeddingHandler(manager)

	body, err := json.Marshal(schema.EmbeddingRequest{
		Provider: conn.Config.Name,
		Model:    modelName,
		Input:    []string{"hello", "world"},
	})
	if err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/embedding", bytes.NewReader(body)).WithContext(newModelHandlerTestContext(t))
	r.Header.Set(types.ContentTypeHeader, types.ContentTypeJSON)
	item.Handler().ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp schema.EmbeddingResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Output) != 2 {
		t.Fatalf("expected 2 embeddings, got %d", len(resp.Output))
	}
}

func TestEmbeddingEmptyInput(t *testing.T) {
	_, manager := newModelHandlerIntegrationManager(t)
	_, _, item := EmbeddingHandler(manager)

	body, err := json.Marshal(schema.EmbeddingRequest{Model: "ignored", Input: []string{}})
	if err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/embedding", bytes.NewReader(body)).WithContext(newModelHandlerTestContext(t))
	r.Header.Set(types.ContentTypeHeader, types.ContentTypeJSON)
	item.Handler().ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestEmbeddingModelNotFound(t *testing.T) {
	conn, manager := newModelHandlerIntegrationManager(t)
	_, _, item := EmbeddingHandler(manager)

	body, err := json.Marshal(schema.EmbeddingRequest{
		Provider: conn.Config.Name,
		Model:    "missing-model",
		Input:    []string{"hello"},
	})
	if err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/embedding", bytes.NewReader(body))
	r.Header.Set(types.ContentTypeHeader, types.ContentTypeJSON)
	item.Handler().ServeHTTP(w, r)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestEmbeddingInvalidJSON(t *testing.T) {
	_, _, item := EmbeddingHandler(nil)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/embedding", bytes.NewBufferString(`{invalid`))
	r.Header.Set(types.ContentTypeHeader, types.ContentTypeJSON)
	item.Handler().ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func requireEmbeddingModel(t *testing.T, conn *llmtest.Conn, manager *llmmanager.Manager) string {
	t.Helper()

	ctx := newModelHandlerTestContext(t)
	models, err := manager.ListModels(ctx, schema.ModelListRequest{Provider: conn.Config.Name}, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, model := range models.Body {
		if model.OwnedBy == conn.Config.Name && model.Cap&schema.ModelCapEmbeddings != 0 {
			return model.Name
		}
	}
	t.Skip("no embedding-capable model available, skipping")
	return ""
}
