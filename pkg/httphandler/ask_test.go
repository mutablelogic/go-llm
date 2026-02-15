package httphandler_test

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	agent "github.com/mutablelogic/go-llm/pkg/agent"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
)

func newGeneratorManager(t *testing.T) *agent.Manager {
	t.Helper()
	client := &mockGeneratorClient{
		mockClient: mockClient{
			name:   "test-provider",
			models: []schema.Model{{Name: "test-model"}},
		},
	}
	return newTestManagerWithGenerator(t, []*mockGeneratorClient{client})
}

func TestAsk_JSON_OK(t *testing.T) {
	mgr := newGeneratorManager(t)
	mux := serveMux(mgr)

	body := `{"model":"test-model","text":"hello"}`
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/ask", bytes.NewBufferString(body))
	r.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp schema.AskResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Role != "assistant" {
		t.Fatalf("expected role=assistant, got %q", resp.Role)
	}
	if len(resp.Content) == 0 {
		t.Fatal("expected at least one content block")
	}
	text := resp.Content[0].Text
	if text == nil || !strings.Contains(*text, "hello") {
		t.Fatalf("expected response to contain 'hello', got %v", text)
	}
}

func TestAsk_JSON_WithProvider(t *testing.T) {
	mgr := newGeneratorManager(t)
	mux := serveMux(mgr)

	body := `{"provider":"test-provider","model":"test-model","text":"hi"}`
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/ask", bytes.NewBufferString(body))
	r.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp schema.AskResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Role != "assistant" {
		t.Fatalf("expected role=assistant, got %q", resp.Role)
	}
}

func TestAsk_JSON_WithBase64Attachment(t *testing.T) {
	mgr := newGeneratorManager(t)
	mux := serveMux(mgr)

	// PNG header bytes
	pngData := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
	encoded := base64.StdEncoding.EncodeToString(pngData)

	body := fmt.Sprintf(`{"model":"test-model","text":"describe this","attachments":[{"type":"image/png","data":"%s"}]}`, encoded)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/ask", bytes.NewBufferString(body))
	r.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp schema.AskResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Role != "assistant" {
		t.Fatalf("expected role=assistant, got %q", resp.Role)
	}
}

func TestAsk_Multipart_OK(t *testing.T) {
	mgr := newGeneratorManager(t)
	mux := serveMux(mgr)

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	// Add text field
	if err := writer.WriteField("text", "describe this image"); err != nil {
		t.Fatal(err)
	}
	// Add model field
	if err := writer.WriteField("model", "test-model"); err != nil {
		t.Fatal(err)
	}
	// Add file
	part, err := writer.CreateFormFile("file", "test.png")
	if err != nil {
		t.Fatal(err)
	}
	// Write some PNG-like bytes
	pngData := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x00, 0x00}
	if _, err := part.Write(pngData); err != nil {
		t.Fatal(err)
	}
	writer.Close()

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/ask", &buf)
	r.Header.Set("Content-Type", writer.FormDataContentType())
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp schema.AskResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Role != "assistant" {
		t.Fatalf("expected role=assistant, got %q", resp.Role)
	}
}

func TestAsk_MethodNotAllowed(t *testing.T) {
	mgr := newGeneratorManager(t)
	mux := serveMux(mgr)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/ask", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAsk_ModelNotFound(t *testing.T) {
	mgr := newGeneratorManager(t)
	mux := serveMux(mgr)

	body := `{"model":"nonexistent","text":"hello"}`
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/ask", bytes.NewBufferString(body))
	r.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(w, r)

	if w.Code == http.StatusOK {
		t.Fatalf("expected error status, got 200")
	}
}

func TestAsk_InvalidJSON(t *testing.T) {
	mgr := newGeneratorManager(t)
	mux := serveMux(mgr)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/ask", bytes.NewBufferString(`{invalid`))
	r.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(w, r)

	if w.Code == http.StatusOK {
		t.Fatalf("expected error status for invalid JSON, got 200")
	}
}
