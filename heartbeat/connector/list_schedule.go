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
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type ListSchedule struct {
	tool.Base
	Connector *Connector
}

type ListScheduleRequest struct{}

var _ llm.Tool = (*ListSchedule)(nil)

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

func (c *Connector) NewListSchedule() ListSchedule {
	return ListSchedule{Connector: c}
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

func (ListSchedule) Name() string {
	return "list_schedule"
}

func (ListSchedule) Description() string {
	return `List all heartbeat schedules.`

}

func (ListSchedule) InputSchema() *jsonschema.Schema {
	return jsonschema.MustFor[schema.HeartbeatListRequest]()
}

func (tool ListSchedule) Run(ctx context.Context, input json.RawMessage) (_ any, err error) {
	toolSession := toolkit.SessionFromContext(ctx)
	toolSession.Logger().InfoContext(ctx, "list_schedule called", "input", string(input))

	// TODO: Get the user

	// Parse the request
	var req schema.HeartbeatListRequest
	if err := json.Unmarshal(input, &req); err != nil {
		return nil, err
	} else if req.Fired == nil {
		// Default to listing only upcoming schedules
		req.Fired = types.Ptr(false)
	}

	// Return the list of schedules for the session
	// TODO: User
	list, err := tool.Connector.Manager.List(ctx, req, nil)
	toolSession.Logger().InfoContext(ctx, "list_schedule result", "list", list, "err", err)
	return list, nil
}
