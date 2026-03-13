package toolkit

import (
	"context"
	"log/slog"
	"testing"

	// Packages
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	resource "github.com/mutablelogic/go-llm/pkg/toolkit/resource"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// Session

func Test_Session_001_from_empty_context(t *testing.T) {
	s := SessionFromContext(context.Background())
	if s == nil {
		t.Fatal("expected non-nil session")
	}
	if s.Logger() == nil {
		t.Fatal("expected non-nil logger")
	}
}

func Test_Session_002_with_and_from_context(t *testing.T) {
	tk, _ := New()
	sess := tk.newSession("my_tool", schema.Meta("id", "test-id"), schema.Meta("k", "v"))
	ctx := withSessionContext(context.Background(), sess)
	got := SessionFromContext(ctx)
	if got == nil {
		t.Fatal("expected non-nil session from context")
	}
	if got.ID() != "test-id" {
		t.Fatalf("expected ID %q, got %q", "test-id", got.ID())
	}
}

func Test_Session_003_with_nil_session_is_noop(t *testing.T) {
	ctx := context.Background()
	got := withSessionContext(ctx, nil)
	if got != ctx {
		t.Fatal("expected same context when session is nil")
	}
}

func Test_Session_004_meta(t *testing.T) {
	tk, _ := New()
	sess := tk.newSession("tool", schema.Meta("foo", "bar"))
	if sess.Meta()["foo"] != "bar" {
		t.Fatalf("expected meta foo=bar, got %v", sess.Meta())
	}
}

func Test_Session_005_logger(t *testing.T) {
	tk, _ := New()
	sess := tk.newSession("tool")
	if sess.Logger() == nil {
		t.Fatal("expected non-nil logger")
	}
}

func Test_Session_006_capabilities_and_client_info_nil(t *testing.T) {
	tk, _ := New()
	sess := tk.newSession("tool")
	if sess.ClientInfo() != nil {
		t.Fatalf("expected nil ClientInfo, got %v", sess.ClientInfo())
	}
	if sess.Capabilities() != nil {
		t.Fatalf("expected nil Capabilities, got %v", sess.Capabilities())
	}
}

func Test_Session_007_progress_does_not_error(t *testing.T) {
	tk, _ := New()
	sess := tk.newSession("tool")
	if err := sess.Progress(0.5, 1.0, "halfway"); err != nil {
		t.Fatalf("expected no error from Progress, got %v", err)
	}
}

func Test_Session_008_progress_no_message(t *testing.T) {
	tk, _ := New()
	sess := tk.newSession("tool")
	if err := sess.Progress(0.0, 1.0); err != nil {
		t.Fatalf("expected no error from Progress with no message, got %v", err)
	}
}

func Test_Session_009_progress_too_many_messages(t *testing.T) {
	tk, _ := New()
	sess := tk.newSession("tool")
	if err := sess.Progress(0.5, 1.0, "a", "b"); err == nil {
		t.Fatal("expected error for too many message args")
	}
}

func Test_Session_010_string(t *testing.T) {
	tk, _ := New()
	sess := tk.newSession("tool", schema.Meta("id", "abc"))
	s := sess.(*session).String()
	if s == "" {
		t.Fatal("expected non-empty String()")
	}
}

func Test_Session_011_with_session_context(t *testing.T) {
	ctx := WithSession(context.Background(), "sess-id", schema.Meta("foo", "bar"))
	meta := metaFromContext(ctx)
	found := false
	for _, m := range meta {
		if m.Key == "id" && m.Value == "sess-id" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected id=sess-id in meta, got %v", meta)
	}
}

func Test_Session_012_meta_from_empty_context(t *testing.T) {
	meta := metaFromContext(context.Background())
	if len(meta) != 0 {
		t.Fatalf("expected empty meta from plain context, got %v", meta)
	}
}

///////////////////////////////////////////////////////////////////////////////
// Event constructors

func Test_Event_001_state_change(t *testing.T) {
	state := schema.ConnectorState{Name: types.Ptr("myserver"), Version: types.Ptr("1.0")}
	evt := StateChangeEvent(state)
	if evt.Kind != ConnectorEventStateChange {
		t.Fatalf("expected StateChange, got %v", evt.Kind)
	}
}

func Test_Event_002_tool_list_change(t *testing.T) {
	evt := ToolListChangeEvent()
	if evt.Kind != ConnectorEventToolListChanged {
		t.Fatalf("expected ToolListChanged, got %v", evt.Kind)
	}
}

func Test_Event_003_prompt_list_change(t *testing.T) {
	evt := PromptListChangeEvent()
	if evt.Kind != ConnectorEventPromptListChanged {
		t.Fatalf("expected PromptListChanged, got %v", evt.Kind)
	}
}

func Test_Event_004_resource_list_change(t *testing.T) {
	evt := ResourceListChangeEvent()
	if evt.Kind != ConnectorEventResourceListChanged {
		t.Fatalf("expected ResourceListChanged, got %v", evt.Kind)
	}
}

func Test_Event_005_resource_updated(t *testing.T) {
	evt := ResourceUpdatedEvent("text:greeting")
	if evt.Kind != ConnectorEventResourceUpdated {
		t.Fatalf("expected ResourceUpdated, got %v", evt.Kind)
	}
	if evt.URI != "text:greeting" {
		t.Fatalf("expected URI %q, got %q", "text:greeting", evt.URI)
	}
}

///////////////////////////////////////////////////////////////////////////////
// Options

func Test_Opt_001_with_prompt(t *testing.T) {
	_, err := New(WithPrompt(&mockPrompt{name: "summarize"}))
	if err != nil {
		t.Fatal(err)
	}
}

func Test_Opt_002_with_resource(t *testing.T) {
	r, _ := resource.Text("greeting", "hello")
	_, err := New(WithResource(r))
	if err != nil {
		t.Fatal(err)
	}
}

func Test_Opt_003_with_logger_nil(t *testing.T) {
	tk, err := New(WithLogger(nil))
	if err != nil {
		t.Fatal(err)
	}
	if tk.logger == nil {
		t.Fatal("expected non-nil logger after WithLogger(nil)")
	}
}

func Test_Opt_004_with_logger_custom(t *testing.T) {
	l := slog.Default()
	tk, err := New(WithLogger(l))
	if err != nil {
		t.Fatal(err)
	}
	if tk.logger != l {
		t.Fatal("expected custom logger to be set")
	}
}

func Test_Opt_005_with_tracer_nil(t *testing.T) {
	_, err := New(WithTracer(nil))
	if err != nil {
		t.Fatal(err)
	}
}
