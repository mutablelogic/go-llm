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
		"For RFC 3339 schedules that already include a timezone offset (e.g. 2026-06-01T15:00:00+02:00), " +
		"the timezone is inferred from the timestamp and need not be provided separately. " +
		"For cron schedules without an embedded offset, pass a timezone (IANA name, e.g. Europe/London) " +
		"so the schedule is evaluated in the user's local time; omitting it defaults to UTC. " +
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
		"For RFC 3339 schedules that already include a timezone offset, the timezone is inferred automatically. " +
		"For cron schedules, pass a timezone (IANA name, e.g. Europe/London) " +
		"so the schedule is evaluated in the user's local time; omitting it defaults to UTC. " +
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
	if req.Schedule != "" || req.Timezone != "" {
		var loc *time.Location
		if req.Timezone != "" {
			if req.Timezone == "Local" {
				return nil, llm.ErrBadParameter.With("update_heartbeat: timezone must be a specific IANA name (e.g. Europe/London), not \"Local\"")
			}
			var locErr error
			loc, locErr = time.LoadLocation(req.Timezone)
			if locErr != nil {
				return nil, llm.ErrBadParameter.Withf("update_heartbeat: unknown timezone %q: %v", req.Timezone, locErr)
			}
		}
		schedStr := req.Schedule
		if schedStr == "" {
			// Timezone-only change: fetch the existing schedule string.
			existing, err := t.mgr.store.Get(req.ID)
			if err != nil {
				return nil, err
			}
			schedStr = existing.Schedule.String()
			if loc == nil {
				loc = existing.Schedule.Loc
			}
		}
		s, err := NewTimeSpec(schedStr, loc)
		if err != nil {
			return nil, llm.ErrBadParameter.Withf("update_heartbeat: %v", err)
		}
		schedule = &s
	}

	return t.mgr.store.Update(req.ID, req.Message, schedule)
}
