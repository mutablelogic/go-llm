package heartbeat

import (
	"context"
	"encoding/json"
	"time"

	// Packages
	jsonschema "github.com/google/jsonschema-go/jsonschema"
	llm "github.com/mutablelogic/go-llm"
	tool "github.com/mutablelogic/go-llm/pkg/tool"
)

///////////////////////////////////////////////////////////////////////////////
// TOOL TYPES

type addHeartbeat struct {
	tool.DefaultTool
	mgr *Manager
}
type deleteHeartbeat struct {
	tool.DefaultTool
	mgr *Manager
}
type listHeartbeats struct {
	tool.DefaultTool
	mgr *Manager
}
type updateHeartbeat struct {
	tool.DefaultTool
	mgr *Manager
}

var _ llm.Tool = (*addHeartbeat)(nil)
var _ llm.Tool = (*deleteHeartbeat)(nil)
var _ llm.Tool = (*listHeartbeats)(nil)
var _ llm.Tool = (*updateHeartbeat)(nil)

///////////////////////////////////////////////////////////////////////////////
// add_heartbeat

func (*addHeartbeat) Name() string { return "add_heartbeat" }

func (*addHeartbeat) Description() string {
	return "Schedule a new heartbeat reminder. " +
		"The message is delivered once the specified time is reached. " +
		"Always pass a timezone (IANA name, e.g. Europe/London, America/New_York) " +
		"so the schedule is evaluated in the user's local time. " +
		"Returns the created heartbeat including its ID."
}

func (*addHeartbeat) InputSchema() (*jsonschema.Schema, error) {
	return jsonschema.For[AddHeartbeatRequest](nil)
}

func (t *addHeartbeat) Run(_ context.Context, input json.RawMessage) (any, error) {
	var req AddHeartbeatRequest
	if len(input) > 0 {
		if err := json.Unmarshal(input, &req); err != nil {
			return nil, llm.ErrBadParameter.Withf("add_heartbeat: %v", err)
		}
	}

	if req.Schedule == "" {
		return nil, llm.ErrBadParameter.With("add_heartbeat: schedule is required")
	}
	var loc *time.Location
	if req.Timezone != "" {
		var locErr error
		loc, locErr = time.LoadLocation(req.Timezone)
		if locErr != nil {
			return nil, llm.ErrBadParameter.Withf("add_heartbeat: unknown timezone %q: %v", req.Timezone, locErr)
		}
	}
	schedule, err := NewTimeSpec(req.Schedule, loc)
	if err != nil {
		return nil, llm.ErrBadParameter.Withf("add_heartbeat: %v", err)
	}

	return t.mgr.store.Create(req.Message, schedule)
}

///////////////////////////////////////////////////////////////////////////////
// delete_heartbeat

func (*deleteHeartbeat) Name() string { return "delete_heartbeat" }

func (*deleteHeartbeat) Description() string {
	return "Delete a heartbeat by its ID. The heartbeat is permanently removed."
}

func (*deleteHeartbeat) InputSchema() (*jsonschema.Schema, error) {
	return jsonschema.For[DeleteHeartbeatRequest](nil)
}

func (t *deleteHeartbeat) Run(_ context.Context, input json.RawMessage) (any, error) {
	var req DeleteHeartbeatRequest
	if len(input) > 0 {
		if err := json.Unmarshal(input, &req); err != nil {
			return nil, llm.ErrBadParameter.Withf("delete_heartbeat: %v", err)
		}
	}
	if req.ID == "" {
		return nil, llm.ErrBadParameter.With("delete_heartbeat: id is required")
	}
	if err := t.mgr.store.Delete(req.ID); err != nil {
		return nil, err
	}
	return map[string]string{"id": req.ID, "status": "deleted"}, nil
}

///////////////////////////////////////////////////////////////////////////////
// list_heartbeats

func (*listHeartbeats) Name() string { return "list_heartbeats" }

func (*listHeartbeats) Description() string {
	return "List heartbeats. " +
		"By default only pending (not-yet-fired) heartbeats are returned. " +
		"Set include_fired to true to also see already-delivered heartbeats."
}

func (*listHeartbeats) InputSchema() (*jsonschema.Schema, error) {
	return jsonschema.For[ListHeartbeatsRequest](nil)
}

func (t *listHeartbeats) Run(_ context.Context, input json.RawMessage) (any, error) {
	var req ListHeartbeatsRequest
	if len(input) > 0 {
		if err := json.Unmarshal(input, &req); err != nil {
			return nil, llm.ErrBadParameter.Withf("list_heartbeats: %v", err)
		}
	}

	return t.mgr.store.List(req.IncludeFired)
}

///////////////////////////////////////////////////////////////////////////////
// update_heartbeat

func (*updateHeartbeat) Name() string { return "update_heartbeat" }

func (*updateHeartbeat) Description() string {
	return "Update the message or schedule of an existing heartbeat. " +
		"Rescheduling a fired heartbeat reactivates it. " +
		"If changing the schedule, always pass a timezone (IANA name, e.g. Europe/London) " +
		"so the new schedule is evaluated in the user's local time. " +
		"Omit any field to leave it unchanged."
}

func (*updateHeartbeat) InputSchema() (*jsonschema.Schema, error) {
	return jsonschema.For[UpdateHeartbeatRequest](nil)
}

func (t *updateHeartbeat) Run(_ context.Context, input json.RawMessage) (any, error) {
	var req UpdateHeartbeatRequest
	if len(input) > 0 {
		if err := json.Unmarshal(input, &req); err != nil {
			return nil, llm.ErrBadParameter.Withf("update_heartbeat: %v", err)
		}
	}
	if req.ID == "" {
		return nil, llm.ErrBadParameter.With("update_heartbeat: id is required")
	}

	var schedule *TimeSpec
	if req.Schedule != "" {
		var loc *time.Location
		if req.Timezone != "" {
			var locErr error
			loc, locErr = time.LoadLocation(req.Timezone)
			if locErr != nil {
				return nil, llm.ErrBadParameter.Withf("update_heartbeat: unknown timezone %q: %v", req.Timezone, locErr)
			}
		}
		s, err := NewTimeSpec(req.Schedule, loc)
		if err != nil {
			return nil, llm.ErrBadParameter.Withf("update_heartbeat: %v", err)
		}
		schedule = &s
	}

	return t.mgr.store.Update(req.ID, req.Message, schedule)
}
