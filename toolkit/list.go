package toolkit

import (
	"context"
	"encoding/json"
	"maps"
	"slices"
	"strings"

	// Packages
	otel "github.com/mutablelogic/go-client/pkg/otel"
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/toolkit/schema"
	jsonschema "github.com/mutablelogic/go-server/pkg/jsonschema"
	types "github.com/mutablelogic/go-server/pkg/types"
	attribute "go.opentelemetry.io/otel/attribute"
)

////////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// ListTools returns the tools in the toolkit, or the tools in the namespace
// in the request, in order of canonical name, with their number
func (self *Toolkit) ListTools(ctx context.Context, req schema.ToolListRequest) (_ *schema.ToolList, err error) {
	_, endSpan := otel.StartSpan(self.tracer, ctx, "llm.toolkit.ListTools",
		attribute.String("arg.req", req.String()),
	)
	defer func() { endSpan(err) }()

	// The namespace, if any, must be an identifier
	namespace := strings.TrimSpace(req.Namespace)
	if namespace != "" && !types.IsIdentifier(namespace) {
		return nil, llm.ErrBadParameter.Withf("invalid namespace %q", namespace)
	}

	// The tools in the namespace, or all the tools, in order of canonical name
	self.mu.RLock()
	all := make([]*schema.Tool, 0, len(self.tools))
	for _, name := range slices.Sorted(maps.Keys(self.tools)) {
		if entry := self.tools[name]; namespace == "" || entry.namespace == namespace {
			all = append(all, entry.wire())
		}
	}
	self.mu.RUnlock()

	return &schema.ToolList{ToolListRequest: req, Count: uint64(len(all)), Body: all}, nil
}

////////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// wire returns the tool in the wire format, with its schemas as JSON. A
// schema which can't be encoded is left out.
func (e entry) wire() *schema.Tool {
	return &schema.Tool{
		Name:        e.tool.Name(),
		Canonical:   schema.CanonicalToolName(e.namespace, e.tool.Name()),
		Description: e.tool.Description(),
		Input:       marshalSchema(e.tool.InputSchema()),
		Output:      marshalSchema(e.tool.OutputSchema()),
	}
}

// marshalSchema returns the JSON schema as JSON, or nil if it's nil or can't
// be encoded
func marshalSchema(v *jsonschema.Schema) json.RawMessage {
	if v == nil {
		return nil
	}
	data, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	return data
}
