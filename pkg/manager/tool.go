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
		meta := schema.ToolMeta{
			Name:        t.Name(),
			Description: t.Description(),
		}
		if s, err := t.Schema(); err == nil && s != nil {
			if data, err := json.Marshal(s); err == nil {
				meta.Schema = data
			}
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

// GetTool returns tool metadata by name.
func (m *Manager) GetTool(_ context.Context, name string) (*schema.ToolMeta, error) {
	t := m.toolkit.Lookup(name)
	if t == nil {
		return nil, llm.ErrNotFound.Withf("tool %q", name)
	}

	// Create the response
	meta := &schema.ToolMeta{
		Name:        t.Name(),
		Description: t.Description(),
	}

	// Marshal the JSON schema if available
	if s, err := t.Schema(); err == nil && s != nil {
		if data, err := json.Marshal(s); err == nil {
			meta.Schema = data
		}
	}

	return meta, nil
}
