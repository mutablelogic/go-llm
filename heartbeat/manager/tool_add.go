package heartbeat

import (
	"context"
	"encoding/json"
	"time"

	// Packages
	otel "github.com/mutablelogic/go-client/pkg/otel"
	llm "github.com/mutablelogic/go-llm"
	hschema "github.com/mutablelogic/go-llm/pkg/heartbeat/schema"
	schema "github.com/mutablelogic/go-llm/kernel/schema"
	tool "github.com/mutablelogic/go-llm/pkg/tool"
	session "github.com/mutablelogic/go-llm/pkg/tool/session"
	jsonschema "github.com/mutablelogic/go-server/pkg/jsonschema"
	attribute "go.opentelemetry.io/otel/attribute"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type addHeartbeat struct {
	tool.DefaultTool
	mgr *Manager
}

var _ llm.Tool = addHeartbeat{}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

func (addHeartbeat) Name() string {
	return "add_heartbeat"
}

func (addHeartbeat) Description() string {
	return "Schedule a new heartbeat reminder. " +
		"The message is delivered once the specified time is reached. " +
		"For RFC 3339 schedules that already include a timezone offset (e.g. 2026-06-01T15:00:00+02:00), " +
		"the timezone is inferred from the timestamp and need not be provided separately. " +
		"For cron schedules without an embedded offset, pass a timezone (IANA name, e.g. Europe/London) " +
		"so the schedule is evaluated in the user's local time; omitting it defaults to UTC. " +
		"Returns the created heartbeat including its ID."
}

func (addHeartbeat) InputSchema() *jsonschema.Schema {
	return jsonschema.MustFor[hschema.AddHeartbeatRequest]()
}

func (t addHeartbeat) Run(ctx context.Context, input json.RawMessage) (_ any, err error) {
	var req hschema.AddHeartbeatRequest
	var loc *time.Location

	// OTEL span
	ctx, endSpan := otel.StartSpan(session.FromContext(ctx).Tracer(), ctx, "add_heartbeat", attribute.String("input", string(input)))
	defer func() { endSpan(err) }()

	// Validate parameters
	if len(input) > 0 {
		if err := json.Unmarshal(input, &req); err != nil {
			return nil, schema.ErrBadParameter.Withf("add_heartbeat: %v", err)
		}
	}
	if req.Schedule == "" {
		return nil, schema.ErrBadParameter.With("add_heartbeat: schedule is required")
	}
	if req.Timezone != "" {
		loc, err = time.LoadLocation(req.Timezone)
		if err != nil {
			return nil, schema.ErrBadParameter.Withf("add_heartbeat: unknown timezone %q: %v", req.Timezone, err)
		}
	} else {
		loc = time.UTC
	}

	// Create a new time specification
	schedule, err := hschema.NewTimeSpec(req.Schedule, loc)
	if err != nil {
		return nil, schema.ErrBadParameter.Withf("add_heartbeat: %v", err)
	}

	// Create the heartbeat
	return t.mgr.store.Create(ctx, hschema.HeartbeatMeta{Message: req.Message, Schedule: schedule})
}
