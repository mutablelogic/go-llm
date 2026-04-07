package httphandler

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	// Packages
	llmmanager "github.com/mutablelogic/go-llm/pkg/manager"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	llmtest "github.com/mutablelogic/go-llm/pkg/test"
	types "github.com/mutablelogic/go-server/pkg/types"
)

var modelHandlerConn llmtest.Conn

const modelHandlerIntegrationTimeout = 2 * time.Minute

func TestMain(m *testing.M) {
	llmtest.Main(m, &modelHandlerConn, llmtest.ProviderConfig{
		Name:     "ollama-handler",
		Provider: schema.Ollama,
		Model:    os.Getenv("OLLAMA_MODEL"),
	})
}

func TestModelDownloadJSONIntegration(t *testing.T) {
	modelName := requireDownloadModel(t)
	conn, manager := newModelHandlerIntegrationManager(t)
	_, _, item := ModelHandler(manager)

	body, err := json.Marshal(schema.DownloadModelRequest{
		Provider: conn.Config.Name,
		Name:     modelName,
	})
	if err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/model", bytes.NewReader(body)).WithContext(newModelHandlerTestContext(t))
	r.Header.Set(types.ContentTypeHeader, types.ContentTypeJSON)
	item.Handler().ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var model schema.Model
	if err := json.NewDecoder(w.Body).Decode(&model); err != nil {
		t.Fatal(err)
	}
	if model.Name != modelName {
		t.Fatalf("expected model %q, got %q", modelName, model.Name)
	}
	if model.OwnedBy != conn.Config.Name {
		t.Fatalf("expected provider %q, got %q", conn.Config.Name, model.OwnedBy)
	}
}

func TestModelDownloadStreamIntegration(t *testing.T) {
	modelName := requireDownloadModel(t)
	conn, manager := newModelHandlerIntegrationManager(t)
	_, _, item := ModelHandler(manager)

	body, err := json.Marshal(schema.DownloadModelRequest{
		Provider: conn.Config.Name,
		Name:     modelName,
	})
	if err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/model", bytes.NewReader(body)).WithContext(newModelHandlerTestContext(t))
	r.Header.Set(types.ContentTypeHeader, types.ContentTypeJSON)
	r.Header.Set(types.ContentAcceptHeader, types.ContentTypeTextStream)
	item.Handler().ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if ct := w.Header().Get(types.ContentTypeHeader); ct != types.ContentTypeTextStream {
		t.Fatalf("expected content type %q, got %q", types.ContentTypeTextStream, ct)
	}

	type sseEvent struct{ name, data string }
	var events []sseEvent
	scanner := bufio.NewScanner(w.Body)
	var current sseEvent
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case strings.HasPrefix(line, "event: "):
			current.name = strings.TrimPrefix(line, "event: ")
		case strings.HasPrefix(line, "data: "):
			current.data = strings.TrimPrefix(line, "data: ")
		case line == "":
			if current.name != "" || current.data != "" {
				events = append(events, current)
				current = sseEvent{}
			}
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}

	var sawResult bool
	for _, event := range events {
		if event.name != schema.EventResult {
			continue
		}
		sawResult = true
		var model schema.Model
		if err := json.Unmarshal([]byte(event.data), &model); err != nil {
			t.Fatalf("decode result event: %v", err)
		}
		if model.Name != modelName {
			t.Fatalf("expected model %q, got %q", modelName, model.Name)
		}
	}
	if !sawResult {
		t.Fatalf("expected result event, got %+v", events)
	}
}

func TestModelDownloadNotAcceptable(t *testing.T) {
	modelName := requireDownloadModel(t)
	conn, manager := newModelHandlerIntegrationManager(t)
	_, _, item := ModelHandler(manager)

	body, err := json.Marshal(schema.DownloadModelRequest{
		Provider: conn.Config.Name,
		Name:     modelName,
	})
	if err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/model", bytes.NewReader(body)).WithContext(newModelHandlerTestContext(t))
	r.Header.Set(types.ContentTypeHeader, types.ContentTypeJSON)
	r.Header.Set(types.ContentAcceptHeader, types.ContentTypeTextPlain)
	item.Handler().ServeHTTP(w, r)

	if w.Code != http.StatusNotAcceptable {
		t.Fatalf("expected 406, got %d: %s", w.Code, w.Body.String())
	}
}

func TestModelDeleteNotFound(t *testing.T) {
	conn, manager := newModelHandlerIntegrationManager(t)
	_, _, item := ModelResourceHandler(manager)
	name := fmt.Sprintf("missing-model-%s", t.Name())

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodDelete, "/model/"+name, nil)
	r.SetPathValue("name", name)
	item.Handler().ServeHTTP(w, r)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
	_ = conn
}

func TestModelDeleteWithProviderNotFound(t *testing.T) {
	conn, manager := newModelHandlerIntegrationManager(t)
	_, _, item := ModelProviderResourceHandler(manager)
	name := fmt.Sprintf("missing-model-%s", t.Name())

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodDelete, "/model/"+conn.Config.Name+"/"+name, nil)
	r.SetPathValue("provider", conn.Config.Name)
	r.SetPathValue("name", name)
	item.Handler().ServeHTTP(w, r)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func newModelHandlerIntegrationManager(t *testing.T) (*llmtest.Conn, *llmmanager.Manager) {
	t.Helper()

	conn := modelHandlerConn.Begin(t)
	t.Cleanup(conn.Close)
	conn.RequireProvider(t)
	ctx := newModelHandlerTestContext(t)

	m, err := llmmanager.New(ctx, "test", "0.0.0", conn, llmmanager.WithPassphrase(1, "test1234"))
	if err != nil {
		t.Fatal(err)
	}
	llmtest.RunBackground(t, func(ctx context.Context) error {
		return m.Run(ctx, llmtest.DiscardLogger())
	})
	llmtest.WaitUntil(t, 5*time.Second, func() bool {
		return m.Toolkit != nil
	}, "timed out waiting for llmmanager Run to initialize toolkit")
	if err := m.Exec(ctx, `TRUNCATE llm.provider CASCADE`); err != nil {
		t.Fatal(err)
	}

	if _, err := m.CreateProvider(ctx, conn.ProviderInsert()); err != nil {
		t.Fatal(err)
	}
	if _, _, err := m.SyncProviders(ctx); err != nil {
		t.Fatal(err)
	}

	return conn, m
}

func requireDownloadModel(t *testing.T) string {
	t.Helper()
	if modelHandlerConn.Config.Model == "" {
		t.Skip("OLLAMA_MODEL not set, skipping model download handler test")
	}
	return modelHandlerConn.Config.Model
}

func newModelHandlerTestContext(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), modelHandlerIntegrationTimeout)
	t.Cleanup(cancel)
	return ctx
}
