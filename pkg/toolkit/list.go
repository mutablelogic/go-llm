package toolkit

import (
	// Packages
	"cmp"
	"context"
	"iter"
	"maps"
	"slices"

	llm "github.com/mutablelogic/go-llm"
	"github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type ListType string

type ListRequest struct {
	// Namespace restricts results to a single source.
	// Use "builtin", "user", or a connector name. Empty string returns all.
	Namespace string

	// Type is required and selects which kind of item to list.
	Type ListType

	// Name filters results to items whose name equals this value.
	// Empty string returns all names.
	Name string

	// Pagination.
	Limit  *uint // nil means no limit
	Offset uint
}

type ListResponse struct {
	Tools     []llm.Tool
	Prompts   []llm.Prompt
	Resources []llm.Resource

	// Pagination metadata.
	Count  uint // total items matched (before pagination)
	Offset uint
	Limit  *uint
}

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	ListTypeTools     ListType = "tool"
	ListTypePrompts   ListType = "prompt"
	ListTypeResources ListType = "resource"
)

const (
	listMaxLimit uint = 100
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// List returns items of the requested type matching the request.
// Only the "builtin" namespace is currently supported.
func (tk *toolkit) List(_ context.Context, req ListRequest) (*ListResponse, error) {
	var resp ListResponse

	// Read lock
	tk.mu.RLock()
	defer tk.mu.RUnlock()

	// Append builtin items when no namespace filter or explicitly "builtin".
	if req.Namespace == "" || req.Namespace == NamespaceBuiltin {
		switch req.Type {
		case ListTypeTools:
			resp.Tools = slices.Collect(filterSeq(maps.Values(tk.tools), func(t llm.Tool) bool {
				return req.Name == "" || t.Name() == req.Name
			}))
		case ListTypePrompts:
			resp.Prompts = slices.Collect(filterSeq(maps.Values(tk.prompts), func(p llm.Prompt) bool {
				return req.Name == "" || p.Name() == req.Name
			}))
		case ListTypeResources:
			resp.Resources = slices.Collect(filterSeq(maps.Values(tk.resources), func(r llm.Resource) bool {
				return req.Name == "" || r.URI() == req.Name
			}))
		}
	}

	// TODO: Append connector items when connectors are implemented.

	// TODO: Append user items when user items are implemented.

	// Sort, count, then paginate
	slices.SortFunc(resp.Tools, func(a, b llm.Tool) int {
		return cmp.Compare(a.Name(), b.Name())
	})
	slices.SortFunc(resp.Prompts, func(a, b llm.Prompt) int {
		return cmp.Compare(a.Name(), b.Name())
	})
	slices.SortFunc(resp.Resources, func(a, b llm.Resource) int {
		return cmp.Compare(a.URI(), b.URI())
	})
	resp.Offset = req.Offset
	resp.Count = uint(len(resp.Tools) + len(resp.Prompts) + len(resp.Resources))
	if req.Limit != nil {
		resp.Limit = types.Ptr(min(resp.Count, min(*req.Limit, listMaxLimit)))
	}
	if req.Limit != nil && *req.Limit > 0 {
		resp.Tools = paginateSlice(resp.Tools, req.Offset, req.Limit)
		resp.Prompts = paginateSlice(resp.Prompts, req.Offset, req.Limit)
		resp.Resources = paginateSlice(resp.Resources, req.Offset, req.Limit)
	}

	// Return success
	return types.Ptr(resp), nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

func paginateSlice[T any](items []T, offset uint, limit *uint) []T {
	if offset >= uint(len(items)) {
		return nil
	}
	items = items[offset:]
	effective := listMaxLimit
	if limit != nil && *limit < effective {
		effective = *limit
	}
	if effective < uint(len(items)) {
		items = items[:effective]
	}
	return items
}

func filterSeq[T any](seq iter.Seq[T], keep func(T) bool) iter.Seq[T] {
	return func(yield func(T) bool) {
		for v := range seq {
			if keep(v) && !yield(v) {
				return
			}
		}
	}
}
