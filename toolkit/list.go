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

	// The tools in the namespace, or all the tools, of the type, or any, in
	// order of canonical name
	typ := strings.TrimSpace(req.Type)
	self.mu.RLock()
	all := make([]*schema.Tool, 0, len(self.tools))
	for _, name := range slices.Sorted(maps.Keys(self.tools)) {
		entry := self.tools[name]
		if namespace != "" && entry.namespace != namespace {
			continue
		}
		entryType := schema.NamespaceBuiltin
		if ns, exists := self.namespaces[entry.namespace]; exists {
			entryType = typeOf(ns)
		}
		if typ == "" || entryType == typ {
			all = append(all, entry.wire(entryType))
		}
	}
	self.mu.RUnlock()

	return &schema.ToolList{ToolListRequest: req, Count: uint64(len(all)), Body: all}, nil
}

// ListNamespaces returns the namespaces in the toolkit, in order of name,
// with their descriptions, and how many tools each has, and their number
func (self *Toolkit) ListNamespaces(ctx context.Context) (_ *schema.NamespaceList, err error) {
	_, endSpan := otel.StartSpan(self.tracer, ctx, "llm.toolkit.ListNamespaces")
	defer func() { endSpan(err) }()

	self.mu.RLock()
	defer self.mu.RUnlock()
	tools := make(map[string]uint64, len(self.namespaces))
	for _, entry := range self.tools {
		tools[entry.namespace]++
	}
	all := make([]*schema.Namespace, 0, len(self.namespaces))
	for _, name := range slices.Sorted(maps.Keys(self.namespaces)) {
		all = append(all, &schema.Namespace{Name: name, Type: typeOf(self.namespaces[name]), Description: self.namespaces[name].Description(), Tools: tools[name]})
	}
	return &schema.NamespaceList{Count: uint64(len(all)), Body: all}, nil
}

////////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// wire returns the tool in the wire format, with its namespace's type, its
// title, when it has one, and its schemas as JSON. A schema which can't be
// encoded is left out.
func (e entry) wire(typ string) *schema.Tool {
	tool := &schema.Tool{
		Name:        e.tool.Name(),
		Canonical:   schema.CanonicalToolName(e.namespace, e.tool.Name()),
		Type:        typ,
		Description: e.tool.Description(),
		Input:       marshalSchema(e.tool.InputSchema()),
		Output:      marshalSchema(e.tool.OutputSchema()),
	}
	tool.Title = e.tool.Title()
	return tool
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
