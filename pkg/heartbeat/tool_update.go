package heartbeat

import (
	"context"
	"encoding/json"
	"time"

	// Packages
	jsonschema "github.com/google/jsonschema-go/jsonschema"
	otel "github.com/mutablelogic/go-client/pkg/otel"
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/pkg/heartbeat/schema"
	tool "github.com/mutablelogic/go-llm/pkg/tool"
	session "github.com/mutablelogic/go-llm/pkg/tool/session"
	attribute "go.opentelemetry.io/otel/attribute"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type updateHeartbeat struct {
	tool.DefaultTool
	mgr *Manager
}

var _ llm.Tool = updateHeartbeat{}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

func (updateHeartbeat) Name() string { return "update_heartbeat" }

func (updateHeartbeat) Description() string {
	return "Update the message or schedule of an existing heartbeat. " +
		"Rescheduling a fired heartbeat reactivates it. " +
		"For RFC 3339 schedules that already include a timezone offset, the timezone is inferred automatically. " +
		"For cron schedules, pass a timezone (IANA name, e.g. Europe/London) " +
		"so the schedule is evaluated in the user's local time; omitting it defaults to UTC. " +
		"Omit any field to leave it unchanged."
}

func (updateHeartbeat) InputSchema() (*jsonschema.Schema, error) {
	return jsonschema.For[schema.UpdateHeartbeatRequest](nil)
}

func (t updateHeartbeat) Run(ctx context.Context, input json.RawMessage) (_ any, err error) {
	var req schema.UpdateHeartbeatRequest

	// Otel
	ctx, endSpan := otel.StartSpan(session.FromContext(ctx).Tracer(), ctx, "update_heartbeat", attribute.String("input", string(input)))
	defer func() { endSpan(err) }()

	// Check parameters
	if len(input) > 0 {
		if err := json.Unmarshal(input, &req); err != nil {
			return nil, llm.ErrBadParameter.Withf("update_heartbeat: %v", err)
		}
	}
	if req.ID == "" {
		return nil, llm.ErrBadParameter.With("update_heartbeat: id is required")
	}

	// Update the schedule
	var schedule *schema.TimeSpec
	if req.Schedule != "" || req.Timezone != "" {
		var loc *time.Location
		schedStr := req.Schedule

		// Load timezone if provided
		if req.Timezone != "" {
			if req.Timezone == "Local" {
				return nil, llm.ErrBadParameter.With("update_heartbeat: timezone must be a specific IANA name (e.g. Europe/London), not \"Local\"")
			}
			var err error
			loc, err = time.LoadLocation(req.Timezone)
			if err != nil {
				return nil, llm.ErrBadParameter.Withf("update_heartbeat: unknown timezone %q: %v", req.Timezone, err)
			}
		}

		// For timezone-only change, get existing schedule string
		if schedStr == "" {
			existing, err := t.mgr.store.Get(ctx, req.ID)
			if err != nil {
				return nil, err
			}
			schedStr = existing.Schedule.String()
		}

		// Parse the schedule
		s, err := schema.NewTimeSpec(schedStr, loc)
		if err != nil {
			return nil, llm.ErrBadParameter.Withf("update_heartbeat: %v", err)
		}
		schedule = &s
	}

	// Create updated meta with any provided fields
	meta := schema.HeartbeatMeta{Message: req.Message}
	if schedule != nil {
		meta.Schedule = *schedule
	}

	// Update
	return t.mgr.store.Update(ctx, req.ID, meta)
}
