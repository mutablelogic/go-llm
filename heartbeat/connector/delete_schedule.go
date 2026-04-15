package connector

import (
	"context"
	"encoding/json"

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

type DeleteSchedule struct {
	tool.Base
	Connector *Connector
}

type DeleteScheduleRequest struct {
	schema.HeartbeatSelector `json:",inline" help:"The ID of the heartbeat schedule to delete."`
}

var _ llm.Tool = (*DeleteSchedule)(nil)

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

func (c *Connector) NewDeleteSchedule() DeleteSchedule {
	return DeleteSchedule{Connector: c}
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

func (DeleteSchedule) Name() string {
	return "delete_schedule"
}

func (DeleteSchedule) Description() string {
	return `Delete a heartbeat schedule.`

}

func (DeleteSchedule) InputSchema() *jsonschema.Schema {
	return jsonschema.MustFor[DeleteScheduleRequest]()
}

func (tool DeleteSchedule) Run(ctx context.Context, input json.RawMessage) (any, error) {
	// Parse the session
	session, err := uuid.Parse(toolkit.SessionFromContext(ctx).ID())
	if err != nil {
		return nil, err
	}

	// TODO: Get the user

	// Parse the request
	var req DeleteScheduleRequest
	if err := json.Unmarshal(input, &req); err != nil {
		return nil, err
	}
	// Delete the heartbeat schedule for the session
	if _, err := tool.Connector.Manager.Delete(ctx, session, uuid.UUID(req.HeartbeatSelector)); err != nil {
		return nil, err
	} else {
		return nil, nil
	}
}
