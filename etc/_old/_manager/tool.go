package manager

import (
	"context"
	"encoding/json"
	"sort"

	// Packages
	otel "github.com/mutablelogic/go-client/pkg/otel"
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
	attribute "go.opentelemetry.io/otel/attribute"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// ListTools returns paginated tool metadata.
func (m *Manager) ListTools(ctx context.Context, req schema.ToolListRequest) (result *schema.ToolList, err error) {
	// Otel span
	_, endSpan := otel.StartSpan(m.tracer, ctx, "ListTools",
		attribute.String("request", req.String()),
	)
	defer func() { endSpan(err) }()

	tools := m.toolkit.ListTools(schema.ToolListRequest{})

	// Sort by name for stable ordering
	sort.Slice(tools, func(i, j int) bool { return tools[i].Name() < tools[j].Name() })

	// Build metadata
	all := make([]schema.ToolMeta, 0, len(tools))
	for _, t := range tools {
		meta, err := newToolMeta(t)
		if err != nil {
			return nil, err
		}
		all = append(all, meta)
	}

	// Paginate
	total := uint(len(all))
	start := min(req.Offset, total)
	end := start + types.Value(req.Limit)
	if req.Limit == nil || end > total {
		end = total
	}

	return &schema.ToolList{
		Count:  total,
		Offset: req.Offset,
		Limit:  req.Limit,
		Body:   all[start:end],
	}, nil
}

// CallTool executes a tool by name with the given input and returns the result.
func (m *Manager) CallTool(ctx context.Context, name string, input json.RawMessage) (result *schema.CallToolResponse, err error) {
	// Otel span
	ctx, endSpan := otel.StartSpan(m.tracer, ctx, "CallTool",
		attribute.String("tool", name),
	)
	defer func() { endSpan(err) }()

	toolResult, err := m.toolkit.Run(ctx, name, input)
	if err != nil {
		return nil, err
	}

	// Read the resource content if present
	var data []byte
	if resource, ok := toolResult.(llm.Resource); ok {
		data, err = resource.Read(ctx)
		if err != nil {
			return nil, schema.ErrInternalServerError.Withf("reading tool result: %v", err)
		}
	}

	return &schema.CallToolResponse{
		Tool:   name,
		Result: data,
	}, nil
}

// GetTool returns tool metadata by name.
func (m *Manager) GetTool(ctx context.Context, name string) (result *schema.ToolMeta, err error) {
	// Otel span
	_, endSpan := otel.StartSpan(m.tracer, ctx, "GetTool",
		attribute.String("tool", name),
	)
	defer func() { endSpan(err) }()

	t := m.toolkit.Lookup(name)
	if t == nil {
		return nil, schema.ErrNotFound.Withf("tool %q", name)
	}
	meta, err := newToolMeta(t)
	if err != nil {
		return nil, err
	}
	return &meta, nil
}

func newToolMeta(tool llm.Tool) (schema.ToolMeta, error) {
	meta, err := schema.NewToolMeta(tool.Name(), tool.Description(), tool.InputSchema(), tool.OutputSchema())
	if err != nil {
		return meta, err
	}
	meta.Hints = newToolHints(tool.Meta())
	return meta, nil
}

func newToolHints(meta llm.ToolMeta) *schema.ToolHints {
	hints := &schema.ToolHints{
		Title:           meta.Title,
		ReadOnlyHint:    meta.ReadOnlyHint,
		IdempotentHint:  meta.IdempotentHint,
		DestructiveHint: meta.DestructiveHint,
		OpenWorldHint:   meta.OpenWorldHint,
	}
	if meta.ReadOnlyHint {
		hints.Keywords = append(hints.Keywords, "readonly")
	}
	if meta.IdempotentHint {
		hints.Keywords = append(hints.Keywords, "idempotent")
	}
	if meta.DestructiveHint != nil && *meta.DestructiveHint {
		hints.Keywords = append(hints.Keywords, "destructive")
	}
	if meta.OpenWorldHint != nil && *meta.OpenWorldHint {
		hints.Keywords = append(hints.Keywords, "openworld")
	}
	if hints.Title == "" && !hints.ReadOnlyHint && !hints.IdempotentHint && hints.DestructiveHint == nil && hints.OpenWorldHint == nil && len(hints.Keywords) == 0 {
		return nil
	}
	return hints
}
