package httphandler

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	// Packages
	uuid "github.com/google/uuid"
	schema "github.com/mutablelogic/go-llm/kernel/schema"
	httpresponse "github.com/mutablelogic/go-server/pkg/httpresponse"
	types "github.com/mutablelogic/go-server/pkg/types"
)

type channelFrames struct {
	session   *schema.Session
	deltas    []schema.StreamDelta
	responses []schema.ChatResponse
	errors    []httpresponse.ErrResponse
}

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

	frames := decodeChannelFrames(t, w.Body)
	if frames.session == nil {
		t.Fatal("expected session frame")
	}
	if len(frames.responses) != 1 {
		t.Fatalf("expected 1 final response, got %d", len(frames.responses))
	}
	resp := frames.responses[0]
	if resp.ID == 0 {
		t.Fatal("expected persisted response ID")
	}
	if resp.Session != session.ID {
		t.Fatalf("expected response session %q, got %q", session.ID, resp.Session)
	}
	if resp.Role != schema.RoleAssistant {
		t.Fatalf("expected role %q, got %q", schema.RoleAssistant, resp.Role)
	}
	if len(resp.Content) == 0 || resp.Content[0].Text == nil || *resp.Content[0].Text == "" {
		t.Fatalf("expected assistant text, got %+v", resp.Content)
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

	frames := decodeChannelFrames(t, w.Body)
	if frames.session == nil {
		t.Fatal("expected session frame")
	}
	if !hasChannelErrorCode(frames.errors, http.StatusConflict) {
		t.Fatalf("expected busy error frame code 409, got %+v", frames.errors)
	}
	if len(frames.responses) != 1 {
		t.Fatalf("expected 1 final response, got %d", len(frames.responses))
	}
	resp := frames.responses[0]
	if resp.Role != schema.RoleAssistant {
		t.Fatalf("expected role %q, got %q", schema.RoleAssistant, resp.Role)
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

	frames := decodeChannelFrames(t, w.Body)
	if frames.session == nil {
		t.Fatal("expected session frame")
	}
	if !hasChannelErrorCode(frames.errors, http.StatusBadRequest) {
		t.Fatalf("expected error frame code 400, got %+v", frames.errors)
	}
	if len(frames.responses) != 1 {
		t.Fatalf("expected 1 final response, got %d", len(frames.responses))
	}
	resp := frames.responses[0]
	if resp.Role != schema.RoleAssistant {
		t.Fatalf("expected role %q, got %q", schema.RoleAssistant, resp.Role)
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
	frames := decodeChannelFrames(t, w.Body)
	if frames.session == nil {
		t.Fatal("expected session frame")
	}
	if len(frames.responses) != 1 {
		t.Fatalf("expected 1 final response, got %d", len(frames.responses))
	}
	resp := frames.responses[0]
	if resp.Role != schema.RoleAssistant {
		t.Fatalf("expected role %q, got %q", schema.RoleAssistant, resp.Role)
	}
}

func TestChannelDeduperFiltersRememberedMessages(t *testing.T) {
	deduper := newChannelDeduper(time.Minute)
	userText := "hello"
	assistantText := "world"

	deduper.RememberMessage(&schema.Message{
		Role:    schema.RoleUser,
		Content: []schema.ContentBlock{{Text: &userText}},
	})
	deduper.RememberResponse(schema.ChatResponse{CompletionResponse: schema.CompletionResponse{
		Role:    schema.RoleAssistant,
		Content: []schema.ContentBlock{{Text: &assistantText}},
		Result:  schema.ResultStop,
	}})

	filtered := deduper.Filter([]*schema.Message{
		{Role: schema.RoleUser, Content: []schema.ContentBlock{{Text: &userText}}},
		{Role: schema.RoleAssistant, Content: []schema.ContentBlock{{Text: &assistantText}}, Result: schema.ResultStop},
		{Role: schema.RoleAssistant, Content: []schema.ContentBlock{{Text: types.Ptr("different")}}, Result: schema.ResultStop},
	})

	if len(filtered) != 1 {
		t.Fatalf("expected 1 filtered message, got %d", len(filtered))
	}
	if filtered[0].Role != schema.RoleAssistant {
		t.Fatalf("expected assistant message, got %q", filtered[0].Role)
	}
	if got := filtered[0].Text(); got != "different" {
		t.Fatalf("expected remaining message %q, got %q", "different", got)
	}
}

func TestChannelDeduperFiltersRememberedMessageIDs(t *testing.T) {
	deduper := newChannelDeduper(time.Minute)
	deduper.RememberMessage(&schema.Message{
		ID:      42,
		Role:    schema.RoleAssistant,
		Content: []schema.ContentBlock{{Text: types.Ptr("persisted")}},
		Result:  schema.ResultStop,
	})

	filtered := deduper.Filter([]*schema.Message{
		{ID: 42, Role: schema.RoleAssistant, Content: []schema.ContentBlock{{Text: types.Ptr("persisted")}}, Result: schema.ResultStop},
		{ID: 43, Role: schema.RoleAssistant, Content: []schema.ContentBlock{{Text: types.Ptr("next")}}, Result: schema.ResultStop},
	})

	if len(filtered) != 1 {
		t.Fatalf("expected 1 filtered message, got %d", len(filtered))
	}
	if filtered[0].ID != 43 {
		t.Fatalf("expected remaining message ID %d, got %d", 43, filtered[0].ID)
	}
}

func TestChannelShouldSendMessage(t *testing.T) {
	if !channelShouldSendMessage(&schema.Message{
		Role:    schema.RoleAssistant,
		Content: []schema.ContentBlock{{Text: types.Ptr("hello")}},
	}) {
		t.Fatal("expected text message to be forwarded")
	}
	if channelShouldSendMessage(&schema.Message{
		Role: schema.RoleAssistant,
		Content: []schema.ContentBlock{
			{Text: types.Ptr("Let me check the current date and time for you.")},
			{ToolCall: &schema.ToolCall{ID: "tool_1", Name: "memory__memory_search"}},
		},
		Result: schema.ResultToolCall,
	}) {
		t.Fatal("expected mixed text and tool-call message to be suppressed")
	}
	if channelShouldSendMessage(&schema.Message{
		Role: schema.RoleUser,
		Content: []schema.ContentBlock{{ToolResult: &schema.ToolResult{
			ID:      "tool_1",
			Name:    "mcp_fetch__fetch",
			Content: json.RawMessage(`{"ok":true}`),
		}}},
	}) {
		t.Fatal("expected tool-result-only message to be suppressed")
	}
	if channelShouldSendMessage(&schema.Message{
		Role: schema.RoleAssistant,
		Content: []schema.ContentBlock{{ToolCall: &schema.ToolCall{
			ID:   "tool_1",
			Name: "mcp_fetch__fetch",
		}}},
	}) {
		t.Fatal("expected tool-call-only message to be suppressed")
	}
}

func TestChannelDeduperFiltersPersistedUserRequest(t *testing.T) {
	deduper := newChannelDeduper(time.Minute)
	deduper.RememberMessage(channelRequestMessage(schema.SessionChannelRequest{Text: "fetch it again"}))

	filtered := deduper.Filter([]*schema.Message{
		{Role: schema.RoleUser, Content: []schema.ContentBlock{{Text: types.Ptr("fetch it again")}}, Result: schema.ResultStop},
		{Role: schema.RoleAssistant, Content: []schema.ContentBlock{{Text: types.Ptr("article body")}}, Result: schema.ResultStop},
	})

	if len(filtered) != 1 {
		t.Fatalf("expected 1 filtered message, got %d", len(filtered))
	}
	if filtered[0].Role != schema.RoleAssistant {
		t.Fatalf("expected assistant message, got %q", filtered[0].Role)
	}
}

func TestChannelDeduperConsumesRememberedMessageOnce(t *testing.T) {
	deduper := newChannelDeduper(time.Minute)
	deduper.RememberMessage(channelRequestMessage(schema.SessionChannelRequest{Text: "repeat"}))

	first := deduper.Filter([]*schema.Message{{
		Role:    schema.RoleUser,
		Content: []schema.ContentBlock{{Text: types.Ptr("repeat")}},
		Result:  schema.ResultStop,
	}})
	if len(first) != 0 {
		t.Fatalf("expected first matching replay to be suppressed, got %d messages", len(first))
	}

	second := deduper.Filter([]*schema.Message{{
		Role:    schema.RoleUser,
		Content: []schema.ContentBlock{{Text: types.Ptr("repeat")}},
		Result:  schema.ResultStop,
	}})
	if len(second) != 1 {
		t.Fatalf("expected second matching message to pass through after consumption, got %d messages", len(second))
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
	frames := decodeChannelFrames(t, w.Body)
	if frames.session == nil {
		t.Fatal("expected session frame")
	}
	if len(frames.responses) != 1 {
		t.Fatalf("expected 1 final response, got %d", len(frames.responses))
	}
	resp := frames.responses[0]
	if resp.Role != schema.RoleAssistant {
		t.Fatalf("expected role %q, got %q", schema.RoleAssistant, resp.Role)
	}
}

func decodeChannelFrames(t *testing.T, reader io.Reader) channelFrames {
	t.Helper()

	dec := json.NewDecoder(reader)
	frames := channelFrames{}
	for {
		var raw json.RawMessage
		if err := dec.Decode(&raw); err != nil {
			if err == io.EOF {
				return frames
			}
			t.Fatalf("decode frame: %v", err)
		}

		if session, ok := decodeSessionFrame(raw); ok {
			frames.session = session
			continue
		}
		if errFrame, ok := decodeChannelError(raw); ok {
			frames.errors = append(frames.errors, errFrame)
			continue
		}
		if delta, ok := decodeChannelDelta(raw); ok {
			frames.deltas = append(frames.deltas, delta)
			continue
		}
		if response, ok := decodeChannelResponse(raw); ok {
			frames.responses = append(frames.responses, response)
			continue
		}

		t.Fatalf("unexpected frame: %s", string(raw))
	}
}

func decodeSessionFrame(raw json.RawMessage) (*schema.Session, bool) {
	var session schema.Session
	if err := json.Unmarshal(raw, &session); err != nil {
		return nil, false
	}
	if session.ID == uuid.Nil {
		return nil, false
	}
	return &session, true
}

func decodeChannelError(raw json.RawMessage) (httpresponse.ErrResponse, bool) {
	var errFrame httpresponse.ErrResponse
	if err := json.Unmarshal(raw, &errFrame); err != nil {
		return httpresponse.ErrResponse{}, false
	}
	if errFrame.Code == 0 {
		return httpresponse.ErrResponse{}, false
	}
	return errFrame, true
}

func decodeChannelDelta(raw json.RawMessage) (schema.StreamDelta, bool) {
	var delta schema.StreamDelta
	if err := json.Unmarshal(raw, &delta); err != nil {
		return schema.StreamDelta{}, false
	}
	if delta.Role == "" || delta.Text == "" {
		return schema.StreamDelta{}, false
	}
	return delta, true
}

func decodeChannelResponse(raw json.RawMessage) (schema.ChatResponse, bool) {
	var response schema.ChatResponse
	if err := json.Unmarshal(raw, &response); err != nil {
		return schema.ChatResponse{}, false
	}
	if len(response.Content) == 0 {
		return schema.ChatResponse{}, false
	}
	return response, true
}

func hasChannelErrorCode(frames []httpresponse.ErrResponse, code int) bool {
	for _, frame := range frames {
		if frame.Code == code {
			return true
		}
	}
	return false
}
