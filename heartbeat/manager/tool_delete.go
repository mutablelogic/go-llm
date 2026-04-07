package heartbeat

import (
	"context"
	"encoding/json"

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

func (deleteHeartbeat) InputSchema() *jsonschema.Schema {
	return jsonschema.MustFor[hschema.DeleteHeartbeatRequest]()
}

func (t deleteHeartbeat) Run(ctx context.Context, input json.RawMessage) (_ any, err error) {
	var req hschema.DeleteHeartbeatRequest

	// Otel
	ctx, endSpan := otel.StartSpan(session.FromContext(ctx).Tracer(), ctx, "delete_heartbeat", attribute.String("input", string(input)))
	defer func() { endSpan(err) }()

	// Check parameters
	if len(input) > 0 {
		if err := json.Unmarshal(input, &req); err != nil {
			return nil, schema.ErrBadParameter.Withf("delete_heartbeat: %v", err)
		}
	}
	if req.ID == "" {
		return nil, schema.ErrBadParameter.With("delete_heartbeat: id is required")
	}

	// Delete from store, and return the deleted heartbeat (or nil if not found)
	return t.mgr.store.Delete(ctx, req.ID)
}
