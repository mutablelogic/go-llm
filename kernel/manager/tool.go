package manager

import (
	"context"
	"encoding/json"
	"slices"

	// Packages
	auth "github.com/djthorpe/go-auth/schema/auth"
	otel "github.com/mutablelogic/go-client/pkg/otel"
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/kernel/schema"
	toolkit "github.com/mutablelogic/go-llm/toolkit"
	resource "github.com/mutablelogic/go-llm/toolkit/resource"
	types "github.com/mutablelogic/go-server/pkg/types"
	attribute "go.opentelemetry.io/otel/attribute"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// ListTools returns paginated tool metadata from the current toolkit.
func (m *Manager) ListTools(ctx context.Context, req schema.ToolListRequest, user *auth.User) (result *schema.ToolList, err error) {
	// Otel span
	ctx, endSpan := otel.StartSpan(m.tracer, ctx, "ListTools",
		attribute.String("request", req.String()),
		attribute.String("user", types.Stringify(user)),
	)
	defer func() { endSpan(err) }()

	// Gather list of tools matching the request, and total count for pagination metadata
	matched, count, err := m.listTools(ctx, req, user)
	if err != nil {
		return nil, err
	}

	// Convert to response format
	body := make([]*schema.ToolMeta, 0, len(matched))
	for _, tool := range matched {
		meta, err := newToolMeta(tool)
		if err != nil {
			return nil, err
		}
		body = append(body, &meta)
	}

	// Return success
	return &schema.ToolList{
		ToolListRequest: req,
		Count:           count,
		Body:            body,
	}, nil
}

// GetTool returns tool metadata by name, scoped by the user's accessible namespaces.
func (m *Manager) GetTool(ctx context.Context, name string, user *auth.User) (result *schema.ToolMeta, err error) {
	// Otel span
	ctx, endSpan := otel.StartSpan(m.tracer, ctx, "GetTool",
		attribute.String("name", name),
		attribute.String("user", types.Stringify(user)),
	)
	defer func() { endSpan(err) }()

	// Filter tools by name, which may return multiple matches if the name is not fully qualified with a namespace.
	tools, _, err := m.listTools(ctx, schema.ToolListRequest{Name: []string{name}}, user)
	if err != nil {
		return nil, err
	}

	// Return errors when number of matches is not exactly 1
	if len(tools) == 0 {
		return nil, schema.ErrNotFound.Withf("tool %q", name)
	}
	if len(tools) > 1 {
		return nil, schema.ErrConflict.Withf("multiple tools matched %q; specify a fully-qualified tool name", name)
	}

	// Convert to response format
	meta, err := newToolMeta(tools[0])
	if err != nil {
		return nil, err
	}

	// Return success
	return &meta, nil
}

// CallTool executes a tool by name with the given input, scoped by the user's accessible namespaces.
func (m *Manager) CallTool(ctx context.Context, name string, req schema.CallToolRequest, user *auth.User) (result llm.Resource, err error) {
	// Otel span
	ctx, endSpan := otel.StartSpan(m.tracer, ctx, "CallTool",
		attribute.String("name", name),
		attribute.String("request", req.String()),
		attribute.String("user", types.Stringify(user)),
	)
	defer func() { endSpan(err) }()

	// Filter tools by name, which may return multiple matches if the name is not fully qualified with a namespace.
	tools, _, err := m.listTools(ctx, schema.ToolListRequest{Name: []string{name}}, user)
	if err != nil {
		return nil, err
	}
	if len(tools) == 0 {
		return nil, schema.ErrNotFound.Withf("tool %q", name)
	}
	if len(tools) > 1 {
		return nil, schema.ErrConflict.Withf("multiple tools matched %q; specify a fully-qualified tool name", name)
	}

	// Append the json input as a resource if provided, and call the tool
	var resources []llm.Resource
	if req.Input != nil {
		input, err := resource.JSON("input", json.RawMessage(req.Input))
		if err != nil {
			return nil, err
		}
		resources = append(resources, input)
	}

	// Call the tool and return the resource directly so transport layers can
	// preserve the original content type and payload.
	return m.Toolkit.Call(ctx, tools[0], resources...)
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

func (m *Manager) listTools(ctx context.Context, req schema.ToolListRequest, user *auth.User) ([]llm.Tool, uint, error) {
	var namespaces []string
	if user == nil {
		if req.Namespace != "" {
			namespaces = []string{req.Namespace}
		}
	} else {
		accessible, err := m.toolNamespacesForUser(ctx, user)
		if err != nil {
			return nil, 0, err
		}
		if req.Namespace == "" {
			namespaces = accessible
		} else if slices.Contains(accessible, req.Namespace) {
			namespaces = []string{req.Namespace}
		} else {
			return nil, 0, nil
		}
	}

	listReq := toolkit.ListRequest{
		Type:       toolkit.ListTypeTools,
		Namespaces: namespaces,
		Name:       req.Name,
		Offset:     uint(req.Offset),
	}
	if req.Limit != nil {
		listReq.Limit = types.Ptr(uint(types.Value(req.Limit)))
	}

	resp, err := m.Toolkit.List(ctx, listReq)
	if err != nil {
		return nil, 0, err
	}

	return resp.Tools, resp.Count, nil
}

func (m *Manager) toolNamespacesForUser(ctx context.Context, user *auth.User) ([]string, error) {
	if user == nil {
		return nil, nil
	}

	// Determine namespaces of connectors accessible to the user, which may grant access to
	// tools in those namespaces. Always include the builtin namespace.
	namespaces := []string{schema.BuiltinNamespace}
	req := schema.ConnectorListRequest{}
	for {
		connectors, err := m.ListConnectors(ctx, req, user)
		if err != nil {
			return nil, err
		}
		if len(connectors.Body) == 0 {
			break
		}
		for _, connector := range connectors.Body {
			namespaces = append(namespaces, types.Value(connector.Namespace))
		}
		req.Offset += uint64(len(connectors.Body))
	}

	// Return success
	return namespaces, nil
}

func newToolMeta(tool llm.Tool) (schema.ToolMeta, error) {
	var meta schema.ToolMeta
	meta.Name = tool.Name()
	meta.Description = tool.Description()
	meta.Title = tool.Meta().Title
	if in := tool.InputSchema(); in != nil {
		if bytes, err := in.MarshalJSON(); err != nil {
			return schema.ToolMeta{}, err
		} else {
			meta.Input = schema.JSONSchema(bytes)
		}
	}
	if out := tool.OutputSchema(); out != nil {
		if bytes, err := out.MarshalJSON(); err != nil {
			return schema.ToolMeta{}, err
		} else {
			meta.Output = schema.JSONSchema(bytes)
		}
	}
	if tool.Meta().ReadOnlyHint {
		meta.Hints = append(meta.Hints, "readonly")
	}
	if tool.Meta().IdempotentHint {
		meta.Hints = append(meta.Hints, "idempotent")
	}
	if tool.Meta().DestructiveHint != nil && types.Value(tool.Meta().DestructiveHint) {
		meta.Hints = append(meta.Hints, "destructive")
	}
	if tool.Meta().OpenWorldHint != nil && types.Value(tool.Meta().OpenWorldHint) {
		meta.Hints = append(meta.Hints, "openworld")
	}
	return meta, nil
}
