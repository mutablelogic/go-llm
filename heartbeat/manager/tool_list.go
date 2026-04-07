package heartbeat

import (
	"context"
	"encoding/json"

	// Packages
	otel "github.com/mutablelogic/go-client/pkg/otel"
	llm "github.com/mutablelogic/go-llm"
	hschema "github.com/mutablelogic/go-llm/heartbeat/schema"
	schema "github.com/mutablelogic/go-llm/kernel/schema"
	tool "github.com/mutablelogic/go-llm/pkg/tool"
	session "github.com/mutablelogic/go-llm/pkg/tool/session"
	jsonschema "github.com/mutablelogic/go-server/pkg/jsonschema"
	attribute "go.opentelemetry.io/otel/attribute"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type listHeartbeats struct {
	tool.DefaultTool
	mgr *Manager
}

var _ llm.Tool = listHeartbeats{}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

func (listHeartbeats) Name() string { return "list_heartbeats" }

func (listHeartbeats) Description() string {
	return "List heartbeats. " +
		"By default only pending (not-yet-fired) heartbeats are returned. " +
		"Set include_fired to true to also see already-delivered heartbeats."
}

func (listHeartbeats) InputSchema() *jsonschema.Schema {
	return jsonschema.MustFor[hschema.ListHeartbeatsRequest]()
}

func (t listHeartbeats) Run(ctx context.Context, input json.RawMessage) (_ any, err error) {
	var req hschema.ListHeartbeatsRequest

	// Otel
	ctx, endSpan := otel.StartSpan(session.FromContext(ctx).Tracer(), ctx, "list_heartbeats", attribute.String("input", string(input)))
	defer func() { endSpan(err) }()

	// Check parameters
	if len(input) > 0 {
		if err := json.Unmarshal(input, &req); err != nil {
			return nil, schema.ErrBadParameter.Withf("list_heartbeats: %v", err)
		}
	}

	// Return list from store
	return t.mgr.store.List(ctx, req.IncludeFired)
}
