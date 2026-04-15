package connector

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	// Packages
	uuid "github.com/google/uuid"
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/heartbeat/schema"
	toolkit "github.com/mutablelogic/go-llm/toolkit"
	tool "github.com/mutablelogic/go-llm/toolkit/tool"
	jsonschema "github.com/mutablelogic/go-server/pkg/jsonschema"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type CreateSchedule struct {
	tool.Base
	Connector *Connector
}

type CreateScheduleRequest struct {
	Prompt   string `json:"prompt" help:"A message describing the purpose of the reminder, e.g. 'Remind me to water the plants every week.'"`
	Schedule string `json:"schedule" help:"The schedule for the reminder, either in cron format (e.g. '0 9 * * 1' for every Monday at 9am) or a specific time in RFC3339 format (e.g. '2024-06-01T15:00:00Z')."`
	Timezone string `json:"timezone" help:"The timezone for evaluating the schedule, in IANA format (e.g. 'Europe/London')."`
}

var _ llm.Tool = (*CreateSchedule)(nil)

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

func (c *Connector) NewCreateSchedule() CreateSchedule {
	return CreateSchedule{Connector: c}
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

func (CreateSchedule) Name() string {
	return "create_schedule"
}

func (CreateSchedule) Description() string {
	return `Schedule a new reminder either recurring (using cron format) or at a specific time in RFC3339 format. Always pass a timezone (IANA name, e.g. Europe/London) so the schedule is evaluated in the user's local time.`
}

func (CreateSchedule) InputSchema() *jsonschema.Schema {
	return jsonschema.MustFor[CreateScheduleRequest]()
}

func (tool CreateSchedule) Run(ctx context.Context, input json.RawMessage) (_ any, err error) {
	// Parse the session
	session, err := uuid.Parse(toolkit.SessionFromContext(ctx).ID())
	if err != nil {
		return nil, err
	}

	// Parse the request
	var req CreateScheduleRequest
	if err := json.Unmarshal(input, &req); err != nil {
		return nil, err
	}
	var loc *time.Location
	if req.Timezone != "" {
		if loc, err = time.LoadLocation(req.Timezone); err != nil {
			return nil, fmt.Errorf("invalid timezone %q: %w", req.Timezone, err)
		}
	}
	timespec, err := schema.NewTimeSpec(req.Schedule, loc)
	if err != nil {
		return nil, fmt.Errorf("invalid schedule %q: %w", req.Schedule, err)
	}

	// Create and return the new reminder
	return tool.Connector.Manager.Create(ctx, session, schema.HeartbeatMeta{
		Message:  req.Prompt,
		Schedule: timespec,
	})
}
