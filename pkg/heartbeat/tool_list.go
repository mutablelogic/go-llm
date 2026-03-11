package heartbeat

import (
	"context"
	"encoding/json"

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

func (listHeartbeats) InputSchema() (*jsonschema.Schema, error) {
	return jsonschema.For[schema.ListHeartbeatsRequest](nil)
}

func (t listHeartbeats) Run(ctx context.Context, input json.RawMessage) (_ any, err error) {
	var req schema.ListHeartbeatsRequest

	// Otel
	ctx, endSpan := otel.StartSpan(session.FromContext(ctx).Tracer(), ctx, "list_heartbeats", attribute.String("input", string(input)))
	defer func() { endSpan(err) }()

	// Check parameters
	if len(input) > 0 {
		if err := json.Unmarshal(input, &req); err != nil {
			return nil, llm.ErrBadParameter.Withf("list_heartbeats: %v", err)
		}
	}

	// Return list from store
	return t.mgr.store.List(ctx, req.IncludeFired)
}
