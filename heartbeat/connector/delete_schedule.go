package connector

import (
	"context"
	"encoding/json"

	// Packages
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
	schema.HeartbeatIDSelector `json:",inline" help:"The ID of the heartbeat schedule to delete."`
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

func (tool DeleteSchedule) Run(ctx context.Context, input json.RawMessage) error {
	toolSession := toolkit.SessionFromContext(ctx)
	toolSession.Logger().InfoContext(ctx, "delete_schedule called", "input", string(input))

	// TODO: Get the user

	// Parse the request
	var req DeleteScheduleRequest
	if err := json.Unmarshal(input, &req); err != nil {
		return nil, err
	}
	// Delete the heartbeat schedule for the session
	return tool.Connector.Manager.Delete(ctx, req.HeartbeatIDSelector)
}
