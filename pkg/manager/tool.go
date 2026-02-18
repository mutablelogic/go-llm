package manager

import (
	"context"
	"encoding/json"
	"sort"

	// Packages
	"github.com/mutablelogic/go-client/pkg/otel"
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
	"go.opentelemetry.io/otel/attribute"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// ListTools returns paginated tool metadata.
func (m *Manager) ListTools(ctx context.Context, req schema.ListToolRequest) (result *schema.ListToolResponse, err error) {
	// Otel span
	_, endSpan := otel.StartSpan(m.tracer, ctx, "ListTools",
		attribute.String("request", req.String()),
	)
	defer func() { endSpan(err) }()

	tools := m.toolkit.Tools()

	// Sort by name for stable ordering
	sort.Slice(tools, func(i, j int) bool { return tools[i].Name() < tools[j].Name() })

	// Build metadata
	all := make([]schema.ToolMeta, 0, len(tools))
	for _, t := range tools {
		s, err := t.Schema()
		if err != nil {
			return nil, err
		}
		meta, err := schema.NewToolMeta(t.Name(), t.Description(), s)
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

	return &schema.ListToolResponse{
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

	var toolResult any
	toolResult, err = m.toolkit.Run(ctx, name, input)
	if err != nil {
		return nil, err
	}

	// Marshal the result to JSON
	var data []byte
	data, err = json.Marshal(toolResult)
	if err != nil {
		return nil, llm.ErrInternalServerError.Withf("marshalling tool result: %v", err)
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
		return nil, llm.ErrNotFound.Withf("tool %q", name)
	}
	s, err := t.Schema()
	if err != nil {
		return nil, err
	}
	meta, err := schema.NewToolMeta(t.Name(), t.Description(), s)
	if err != nil {
		return nil, err
	}
	return &meta, nil
}
