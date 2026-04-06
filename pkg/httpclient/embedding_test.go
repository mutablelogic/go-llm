package httpclient_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	// Packages
	httpclient "github.com/mutablelogic/go-llm/pkg/httpclient"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
)

func newEmbeddingServer(t *testing.T) *httptest.Server {
	t.Helper()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/embedding", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req schema.EmbeddingRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		if req.Model == "missing-model" {
			http.Error(w, "model not found", http.StatusNotFound)
			return
		}

		output := make([][]float64, len(req.Input))
		for i := range req.Input {
			output[i] = []float64{0.1, 0.2, 0.3}
		}

		response := schema.EmbeddingResponse{
			EmbeddingRequest: schema.EmbeddingRequest{
				Provider:             req.Provider,
				Model:                req.Model,
				Input:                req.Input,
				TaskType:             req.TaskType,
				Title:                req.Title,
				OutputDimensionality: 3,
			},
			Output: output,
			Usage:  &schema.UsageMeta{InputTokens: 5},
		}

		w.Header().Set(types.ContentTypeHeader, types.ContentTypeJSON)
		_ = json.NewEncoder(w).Encode(response)
	})

	return httptest.NewServer(mux)
}

func newEmbeddingClient(t *testing.T, serverURL string) *httpclient.Client {
	t.Helper()

	client, err := httpclient.New(serverURL + "/api")
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func TestEmbeddingJSON(t *testing.T) {
	server := newEmbeddingServer(t)
	defer server.Close()

	client := newEmbeddingClient(t, server.URL)
	response, err := client.Embedding(context.Background(), schema.EmbeddingRequest{
		Provider: "test-provider",
		Model:    "embed-model",
		Input:    []string{"hello world"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if response.Provider != "test-provider" {
		t.Fatalf("expected provider %q, got %q", "test-provider", response.Provider)
	}
	if response.Model != "embed-model" {
		t.Fatalf("expected model %q, got %q", "embed-model", response.Model)
	}
	if len(response.Output) != 1 || len(response.Output[0]) != 3 {
		t.Fatalf("unexpected output: %+v", response.Output)
	}
	if response.Usage == nil || response.Usage.InputTokens != 5 {
		t.Fatalf("unexpected usage: %+v", response.Usage)
	}
}

func TestEmbeddingBatch(t *testing.T) {
	server := newEmbeddingServer(t)
	defer server.Close()

	client := newEmbeddingClient(t, server.URL)
	response, err := client.Embedding(context.Background(), schema.EmbeddingRequest{
		Model: "embed-model",
		Input: []string{"hello", "world"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(response.Output) != 2 {
		t.Fatalf("expected 2 embeddings, got %d", len(response.Output))
	}
}

func TestEmbeddingEmptyModel(t *testing.T) {
	server := newEmbeddingServer(t)
	defer server.Close()

	client := newEmbeddingClient(t, server.URL)
	_, err := client.Embedding(context.Background(), schema.EmbeddingRequest{
		Input: []string{"hello"},
	})
	if err == nil {
		t.Fatal("expected error for empty model")
	}
}

func TestEmbeddingEmptyInput(t *testing.T) {
	server := newEmbeddingServer(t)
	defer server.Close()

	client := newEmbeddingClient(t, server.URL)
	_, err := client.Embedding(context.Background(), schema.EmbeddingRequest{
		Model: "embed-model",
	})
	if err == nil {
		t.Fatal("expected error for empty input")
	}
}

func TestEmbeddingNotFound(t *testing.T) {
	server := newEmbeddingServer(t)
	defer server.Close()

	client := newEmbeddingClient(t, server.URL)
	_, err := client.Embedding(context.Background(), schema.EmbeddingRequest{
		Model: "missing-model",
		Input: []string{"hello"},
	})
	if err == nil {
		t.Fatal("expected error for missing model")
	}
}

func TestEmbeddingMalformedJSONResponse(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/embedding", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set(types.ContentTypeHeader, types.ContentTypeJSON)
		_, _ = w.Write(bytes.TrimSpace([]byte(`{"output":`)))
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	client := newEmbeddingClient(t, server.URL)
	_, err := client.Embedding(context.Background(), schema.EmbeddingRequest{
		Model: "embed-model",
		Input: []string{"hello"},
	})
	if err == nil {
		t.Fatal("expected error for malformed response")
	}
}
