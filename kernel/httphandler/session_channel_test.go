package httphandler

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	// Packages
	uuid "github.com/google/uuid"
	schema "github.com/mutablelogic/go-llm/kernel/schema"
	httpresponse "github.com/mutablelogic/go-server/pkg/httpresponse"
	types "github.com/mutablelogic/go-server/pkg/types"
)

func TestSessionChannelIntegration(t *testing.T) {
	modelName := requireDownloadModel(t)
	conn, manager := newModelHandlerIntegrationManager(t)
	_, _, item := SessionChannelHandler(manager)
	session := createChatTestSession(t, manager, conn.Config.Name, modelName)

	frame, err := json.Marshal(schema.SessionChannelRequest{Text: "Say hello in exactly three words."})
	if err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/session/"+session.ID.String()+"/channel", bytes.NewReader(append(frame, '\n'))).WithContext(newModelHandlerTestContext(t))
	r.SetPathValue("session", session.ID.String())
	r.Header.Set(types.ContentTypeHeader, types.ContentTypeJSONStream)
	r.Header.Set(types.ContentAcceptHeader, types.ContentTypeJSONStream)
	item.Handler().ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if ct := w.Header().Get(types.ContentTypeHeader); ct != types.ContentTypeJSONStream {
		t.Fatalf("expected content type %q, got %q", types.ContentTypeJSONStream, ct)
	}

	var resp schema.ChatResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Role != schema.RoleAssistant {
		t.Fatalf("expected role %q, got %q", schema.RoleAssistant, resp.Role)
	}
	if len(resp.Content) == 0 || resp.Content[0].Text == nil || *resp.Content[0].Text == "" {
		t.Fatalf("expected assistant text, got %+v", resp.Content)
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != io.EOF {
		t.Fatalf("expected EOF after first response, got %v", err)
	}
}

func TestSessionChannelMultipleFramesIntegration(t *testing.T) {
	modelName := requireDownloadModel(t)
	conn, manager := newModelHandlerIntegrationManager(t)
	_, _, item := SessionChannelHandler(manager)
	session := createChatTestSession(t, manager, conn.Config.Name, modelName)

	frame1, err := json.Marshal(schema.SessionChannelRequest{Text: "Say hello in exactly three words."})
	if err != nil {
		t.Fatal(err)
	}
	frame2, err := json.Marshal(schema.SessionChannelRequest{Text: "Say goodbye in exactly three words."})
	if err != nil {
		t.Fatal(err)
	}
	body := append(append(frame1, '\n'), append(frame2, '\n')...)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/session/"+session.ID.String()+"/channel", bytes.NewReader(body)).WithContext(newModelHandlerTestContext(t))
	r.SetPathValue("session", session.ID.String())
	r.Header.Set(types.ContentTypeHeader, types.ContentTypeJSONStream)
	r.Header.Set(types.ContentAcceptHeader, types.ContentTypeJSONStream)
	item.Handler().ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	dec := json.NewDecoder(w.Body)
	for i := 0; i < 2; i++ {
		var resp schema.ChatResponse
		if err := dec.Decode(&resp); err != nil {
			t.Fatalf("decode response %d: %v", i+1, err)
		}
		if resp.Role != schema.RoleAssistant {
			t.Fatalf("response %d: expected role %q, got %q", i+1, schema.RoleAssistant, resp.Role)
		}
	}
	var extra schema.ChatResponse
	if err := dec.Decode(&extra); err != io.EOF {
		t.Fatalf("expected EOF after two responses, got %v", err)
	}
}

func TestSessionChannelInvalidSession(t *testing.T) {
	_, _, item := SessionChannelHandler(nil)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/session/not-a-uuid/channel", bytes.NewBufferString("{}\n"))
	r.SetPathValue("session", "not-a-uuid")
	r.Header.Set(types.ContentTypeHeader, types.ContentTypeJSONStream)
	r.Header.Set(types.ContentAcceptHeader, types.ContentTypeJSONStream)
	item.Handler().ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSessionChannelSessionNotFound(t *testing.T) {
	_, manager := newModelHandlerIntegrationManager(t)
	_, _, item := SessionChannelHandler(manager)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/session/00000000-0000-0000-0000-000000000001/channel", bytes.NewBufferString("{}\n")).WithContext(newModelHandlerTestContext(t))
	r.SetPathValue("session", "00000000-0000-0000-0000-000000000001")
	r.Header.Set(types.ContentTypeHeader, types.ContentTypeJSONStream)
	r.Header.Set(types.ContentAcceptHeader, types.ContentTypeJSONStream)
	item.Handler().ServeHTTP(w, r)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSessionChannelInvalidAccept(t *testing.T) {
	modelName := requireDownloadModel(t)
	conn, manager := newModelHandlerIntegrationManager(t)
	_, _, item := SessionChannelHandler(manager)
	session := createChatTestSession(t, manager, conn.Config.Name, modelName)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/session/"+session.ID.String()+"/channel", bytes.NewBufferString("{}\n")).WithContext(newModelHandlerTestContext(t))
	r.SetPathValue("session", session.ID.String())
	r.Header.Set(types.ContentTypeHeader, types.ContentTypeJSONStream)
	r.Header.Set(types.ContentAcceptHeader, types.ContentTypeTextPlain)
	item.Handler().ServeHTTP(w, r)

	if w.Code != http.StatusNotAcceptable {
		t.Fatalf("expected 406, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSessionChannelFrameErrorContinues(t *testing.T) {
	modelName := requireDownloadModel(t)
	conn, manager := newModelHandlerIntegrationManager(t)
	_, _, item := SessionChannelHandler(manager)
	session := createChatTestSession(t, manager, conn.Config.Name, modelName)

	good, err := json.Marshal(schema.SessionChannelRequest{Text: "Say hello in exactly three words."})
	if err != nil {
		t.Fatal(err)
	}
	body := append([]byte(`{"text":123}`+"\n"), append(good, '\n')...)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/session/"+session.ID.String()+"/channel", bytes.NewReader(body)).WithContext(newModelHandlerTestContext(t))
	r.SetPathValue("session", session.ID.String())
	r.Header.Set(types.ContentTypeHeader, types.ContentTypeJSONStream)
	r.Header.Set(types.ContentAcceptHeader, types.ContentTypeJSONStream)
	item.Handler().ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	dec := json.NewDecoder(w.Body)
	var errFrame httpresponse.ErrResponse
	if err := dec.Decode(&errFrame); err != nil {
		t.Fatal(err)
	}
	if errFrame.Code != http.StatusBadRequest {
		t.Fatalf("expected error frame code 400, got %d (%+v)", errFrame.Code, errFrame)
	}

	var resp schema.ChatResponse
	if err := dec.Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Role != schema.RoleAssistant {
		t.Fatalf("expected role %q, got %q", schema.RoleAssistant, resp.Role)
	}
	if err := dec.Decode(&resp); err != io.EOF {
		t.Fatalf("expected EOF after error and response frames, got %v", err)
	}
}

func TestSessionChannelInvalidContentType(t *testing.T) {
	modelName := requireDownloadModel(t)
	conn, manager := newModelHandlerIntegrationManager(t)
	_, _, item := SessionChannelHandler(manager)
	session := createChatTestSession(t, manager, conn.Config.Name, modelName)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/session/"+session.ID.String()+"/channel", bytes.NewBufferString("{}\n")).WithContext(newModelHandlerTestContext(t))
	r.SetPathValue("session", session.ID.String())
	r.Header.Set(types.ContentTypeHeader, types.ContentTypeJSON)
	r.Header.Set(types.ContentAcceptHeader, types.ContentTypeJSONStream)
	item.Handler().ServeHTTP(w, r)

	if w.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("expected 415, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSessionChannelAcceptsWildcard(t *testing.T) {
	modelName := requireDownloadModel(t)
	conn, manager := newModelHandlerIntegrationManager(t)
	_, _, item := SessionChannelHandler(manager)
	session := createChatTestSession(t, manager, conn.Config.Name, modelName)

	frame, err := json.Marshal(schema.SessionChannelRequest{Text: "Say hello in exactly three words."})
	if err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/session/"+session.ID.String()+"/channel", bytes.NewReader(append(frame, '\n'))).WithContext(newModelHandlerTestContext(t))
	r.SetPathValue("session", session.ID.String())
	r.Header.Set(types.ContentTypeHeader, types.ContentTypeJSONStream)
	r.Header.Set(types.ContentAcceptHeader, types.ContentTypeAny)
	item.Handler().ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp schema.ChatResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Role != schema.RoleAssistant {
		t.Fatalf("expected role %q, got %q", schema.RoleAssistant, resp.Role)
	}
}

func TestSessionChannelSessionPathOwnsSession(t *testing.T) {
	modelName := requireDownloadModel(t)
	conn, manager := newModelHandlerIntegrationManager(t)
	_, _, item := SessionChannelHandler(manager)
	session := createChatTestSession(t, manager, conn.Config.Name, modelName)

	wrongSession := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	frame, err := json.Marshal(map[string]any{
		"session": wrongSession,
		"text":    "Say hello in exactly three words.",
	})
	if err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/session/"+session.ID.String()+"/channel", bytes.NewReader(append(frame, '\n'))).WithContext(newModelHandlerTestContext(t))
	r.SetPathValue("session", session.ID.String())
	r.Header.Set(types.ContentTypeHeader, types.ContentTypeJSONStream)
	r.Header.Set(types.ContentAcceptHeader, types.ContentTypeJSONStream)
	item.Handler().ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp schema.ChatResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Role != schema.RoleAssistant {
		t.Fatalf("expected role %q, got %q", schema.RoleAssistant, resp.Role)
	}
}
