package heartbeat_test

import (
	"context"
	"encoding/json"
	"testing"

	// Packages
	heartbeat "github.com/mutablelogic/go-llm/pkg/heartbeat"
	assert "github.com/stretchr/testify/assert"
)

// newTestManager creates a Manager backed by a temporary store.
func newTestManager(t *testing.T) *heartbeat.Manager {
	t.Helper()
	s := newTestStore(t)
	mgr, err := heartbeat.New(s)
	if err != nil {
		t.Fatal(err)
	}
	return mgr
}

// toolByName returns the named tool from the manager, or fails the test.
func toolByName(t *testing.T, mgr *heartbeat.Manager, name string) interface {
	Run(context.Context, json.RawMessage) (any, error)
} {
	t.Helper()
	tools, err := mgr.ListTools(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range tools {
		if tool.Name() == name {
			return tool
		}
	}
	t.Fatalf("tool %q not found", name)
	return nil
}

func rawJSON(t *testing.T, v any) json.RawMessage {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

///////////////////////////////////////////////////////////////////////////////
// add_heartbeat

func Test_tool_001(t *testing.T) {
	// missing schedule: error
	assert := assert.New(t)
	mgr := newTestManager(t)
	tool := toolByName(t, mgr, "add_heartbeat")
	_, err := tool.Run(context.Background(), rawJSON(t, map[string]any{
		"message": "no schedule",
	}))
	assert.Error(err)
}

func Test_tool_002(t *testing.T) {
	// invalid timezone: error
	assert := assert.New(t)
	mgr := newTestManager(t)
	tool := toolByName(t, mgr, "add_heartbeat")
	_, err := tool.Run(context.Background(), rawJSON(t, map[string]any{
		"message":  "bad tz",
		"schedule": "* * * * *",
		"timezone": "Not/ATimezone",
	}))
	assert.Error(err)
}

func Test_tool_003(t *testing.T) {
	// "Local" timezone: error
	assert := assert.New(t)
	mgr := newTestManager(t)
	tool := toolByName(t, mgr, "add_heartbeat")
	_, err := tool.Run(context.Background(), rawJSON(t, map[string]any{
		"message":  "local tz",
		"schedule": "* * * * *",
		"timezone": "Local",
	}))
	assert.Error(err)
}

func Test_tool_004(t *testing.T) {
	// valid add: returns heartbeat with ID
	assert := assert.New(t)
	mgr := newTestManager(t)
	tool := toolByName(t, mgr, "add_heartbeat")
	result, err := tool.Run(context.Background(), rawJSON(t, map[string]any{
		"message":  "remind me",
		"schedule": "* * * * *",
		"timezone": "Europe/London",
	}))
	assert.NoError(err)
	h, ok := result.(*heartbeat.Heartbeat)
	assert.True(ok)
	assert.NotEmpty(h.ID)
	assert.Equal("remind me", h.Message)
	assert.Equal("Europe/London", h.Schedule.Loc.String())
}

///////////////////////////////////////////////////////////////////////////////
// list_heartbeats

func Test_tool_005(t *testing.T) {
	// empty store: returns empty slice
	assert := assert.New(t)
	mgr := newTestManager(t)
	tool := toolByName(t, mgr, "list_heartbeats")
	result, err := tool.Run(context.Background(), nil)
	assert.NoError(err)
	list, ok := result.([]*heartbeat.Heartbeat)
	assert.True(ok)
	assert.Empty(list)
}

func Test_tool_006(t *testing.T) {
	// after add: list returns it
	assert := assert.New(t)
	mgr := newTestManager(t)
	addTool := toolByName(t, mgr, "add_heartbeat")
	listTool := toolByName(t, mgr, "list_heartbeats")
	addTool.Run(context.Background(), rawJSON(t, map[string]any{
		"message":  "hello",
		"schedule": "* * * * *",
	}))
	result, err := listTool.Run(context.Background(), nil)
	assert.NoError(err)
	list, ok := result.([]*heartbeat.Heartbeat)
	assert.True(ok)
	assert.Len(list, 1)
}

///////////////////////////////////////////////////////////////////////////////
// delete_heartbeat

func Test_tool_007(t *testing.T) {
	// delete non-existent: error
	assert := assert.New(t)
	mgr := newTestManager(t)
	tool := toolByName(t, mgr, "delete_heartbeat")
	_, err := tool.Run(context.Background(), rawJSON(t, map[string]any{
		"id": "no-such-id",
	}))
	assert.Error(err)
}

func Test_tool_008(t *testing.T) {
	// delete existing: succeeds; no longer in list
	assert := assert.New(t)
	mgr := newTestManager(t)
	addTool := toolByName(t, mgr, "add_heartbeat")
	deleteTool := toolByName(t, mgr, "delete_heartbeat")
	listTool := toolByName(t, mgr, "list_heartbeats")
	res, _ := addTool.Run(context.Background(), rawJSON(t, map[string]any{
		"message":  "delete me",
		"schedule": "* * * * *",
	}))
	h := res.(*heartbeat.Heartbeat)
	_, err := deleteTool.Run(context.Background(), rawJSON(t, map[string]any{"id": h.ID}))
	assert.NoError(err)
	result, _ := listTool.Run(context.Background(), nil)
	list := result.([]*heartbeat.Heartbeat)
	assert.Empty(list)
}

///////////////////////////////////////////////////////////////////////////////
// update_heartbeat

func Test_tool_009(t *testing.T) {
	// missing id: error
	assert := assert.New(t)
	mgr := newTestManager(t)
	tool := toolByName(t, mgr, "update_heartbeat")
	_, err := tool.Run(context.Background(), rawJSON(t, map[string]any{
		"message": "no id",
	}))
	assert.Error(err)
}

func Test_tool_010(t *testing.T) {
	// update message only
	assert := assert.New(t)
	mgr := newTestManager(t)
	addTool := toolByName(t, mgr, "add_heartbeat")
	updateTool := toolByName(t, mgr, "update_heartbeat")
	res, _ := addTool.Run(context.Background(), rawJSON(t, map[string]any{
		"message":  "old message",
		"schedule": "* * * * *",
	}))
	h := res.(*heartbeat.Heartbeat)
	updated, err := updateTool.Run(context.Background(), rawJSON(t, map[string]any{
		"id":      h.ID,
		"message": "new message",
	}))
	assert.NoError(err)
	u := updated.(*heartbeat.Heartbeat)
	assert.Equal("new message", u.Message)
}

func Test_tool_011(t *testing.T) {
	// timezone-only update: timezone persisted
	assert := assert.New(t)
	mgr := newTestManager(t)
	addTool := toolByName(t, mgr, "add_heartbeat")
	updateTool := toolByName(t, mgr, "update_heartbeat")
	res, _ := addTool.Run(context.Background(), rawJSON(t, map[string]any{
		"message":  "tz test",
		"schedule": "* * * * *",
	}))
	h := res.(*heartbeat.Heartbeat)
	updated, err := updateTool.Run(context.Background(), rawJSON(t, map[string]any{
		"id":       h.ID,
		"timezone": "Europe/Paris",
	}))
	assert.NoError(err)
	u := updated.(*heartbeat.Heartbeat)
	assert.NotNil(u.Schedule.Loc)
	assert.Equal("Europe/Paris", u.Schedule.Loc.String())
}

func Test_tool_012(t *testing.T) {
	// "Local" timezone in update: error
	assert := assert.New(t)
	mgr := newTestManager(t)
	addTool := toolByName(t, mgr, "add_heartbeat")
	updateTool := toolByName(t, mgr, "update_heartbeat")
	res, _ := addTool.Run(context.Background(), rawJSON(t, map[string]any{
		"message":  "tz test",
		"schedule": "* * * * *",
	}))
	h := res.(*heartbeat.Heartbeat)
	_, err := updateTool.Run(context.Background(), rawJSON(t, map[string]any{
		"id":       h.ID,
		"timezone": "Local",
	}))
	assert.Error(err)
}
