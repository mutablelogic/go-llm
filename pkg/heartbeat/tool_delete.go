package heartbeat

import (
	"context"
	"encoding/json"

	// Packages
	jsonschema "github.com/google/jsonschema-go/jsonschema"
	otel "github.com/mutablelogic/go-client/pkg/otel"
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/pkg/heartbeat/schema"
	server "github.com/mutablelogic/go-llm/pkg/mcp/server"
	tool "github.com/mutablelogic/go-llm/pkg/tool"
	attribute "go.opentelemetry.io/otel/attribute"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type deleteHeartbeat struct {
	tool.DefaultTool
	mgr *Manager
}

var _ llm.Tool = deleteHeartbeat{}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

func (deleteHeartbeat) Name() string {
	return "delete_heartbeat"
}

func (deleteHeartbeat) Description() string {
	return "Delete a heartbeat by its ID. The heartbeat is permanently removed."
}

func (deleteHeartbeat) InputSchema() (*jsonschema.Schema, error) {
	return jsonschema.For[schema.DeleteHeartbeatRequest](nil)
}

func (t deleteHeartbeat) Run(ctx context.Context, input json.RawMessage) (_ any, err error) {
	var req schema.DeleteHeartbeatRequest

	// Otel
	ctx, endSpan := otel.StartSpan(server.SessionFromContext(ctx).Tracer(), ctx, "delete_heartbeat", attribute.String("input", string(input)))
	defer func() { endSpan(err) }()

	// Check parameters
	if len(input) > 0 {
		if err := json.Unmarshal(input, &req); err != nil {
			return nil, llm.ErrBadParameter.Withf("delete_heartbeat: %v", err)
		}
	}
	if req.ID == "" {
		return nil, llm.ErrBadParameter.With("delete_heartbeat: id is required")
	}

	// Delete from store, and return the deleted heartbeat (or nil if not found)
	return t.mgr.store.Delete(ctx, req.ID)
}
