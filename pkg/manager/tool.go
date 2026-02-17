package manager

import (
	"context"
	"encoding/json"
	"sort"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// ListTools returns paginated tool metadata.
func (m *Manager) ListTools(_ context.Context, req schema.ListToolRequest) (*schema.ListToolResponse, error) {
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
	start := req.Offset
	if start > total {
		start = total
	}
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
func (m *Manager) CallTool(ctx context.Context, name string, input json.RawMessage) (*schema.CallToolResponse, error) {
	result, err := m.toolkit.Run(ctx, name, input)
	if err != nil {
		return nil, err
	}

	// Marshal the result to JSON
	data, err := json.Marshal(result)
	if err != nil {
		return nil, llm.ErrInternalServerError.Withf("marshalling tool result: %v", err)
	}

	return &schema.CallToolResponse{
		Tool:   name,
		Result: data,
	}, nil
}

// GetTool returns tool metadata by name.
func (m *Manager) GetTool(_ context.Context, name string) (*schema.ToolMeta, error) {
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
