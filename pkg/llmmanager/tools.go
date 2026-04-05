package manager

import (
	"context"

	// Packages
	otel "github.com/mutablelogic/go-client/pkg/otel"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	toolkit "github.com/mutablelogic/go-llm/pkg/toolkit"
	types "github.com/mutablelogic/go-server/pkg/types"
	attribute "go.opentelemetry.io/otel/attribute"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// ListTools returns paginated tool metadata from the current toolkit.
func (m *Manager) ListTools(ctx context.Context, req schema.ListToolRequest) (result *schema.ListToolResponse, err error) {
	ctx, endSpan := otel.StartSpan(m.tracer, ctx, "ListTools",
		attribute.String("request", req.String()),
	)
	defer func() { endSpan(err) }()

	listReq := toolkit.ListRequest{
		Type:   toolkit.ListTypeTools,
		Offset: uint(req.Offset),
	}
	if req.Limit != nil {
		listReq.Limit = types.Ptr(uint(types.Value(req.Limit)))
	}

	tools, err := m.Toolkit.List(ctx, listReq)
	if err != nil {
		return nil, err
	}

	body := make([]schema.ToolMeta, 0, len(tools.Tools))
	for _, tool := range tools.Tools {
		inputSchema := tool.InputSchema()
		meta, err := schema.NewToolMeta(tool.Name(), tool.Description(), inputSchema)
		if err != nil {
			return nil, err
		}
		body = append(body, meta)
	}

	return &schema.ListToolResponse{
		ListToolRequest: req,
		Count:           tools.Count,
		Body:            body,
	}, nil
}
