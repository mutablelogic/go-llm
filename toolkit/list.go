package toolkit

import (
	// Packages
	"cmp"
	"context"
	"iter"
	"maps"
	"slices"
	"strings"
	"sync"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/kernel/schema"
	prompt "github.com/mutablelogic/go-llm/toolkit/prompt"
	resource "github.com/mutablelogic/go-llm/toolkit/resource"
	tool "github.com/mutablelogic/go-llm/toolkit/tool"
	types "github.com/mutablelogic/go-server/pkg/types"
	errgroup "golang.org/x/sync/errgroup"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type ListType string

type ListRequest struct {
	// Namespaces restrict results to specific sources.
	// Use "builtin", "user", or connector names. Empty means all sources.
	Namespaces []string

	// Type is required and selects which kind of item to list.
	Type ListType

	// Name filters results to specific item names.
	// Empty means no name filter. Qualified names match exactly
	// (for example "builtin.alpha"); bare names match any namespace
	// whose underlying item name equals the filter (for example "alpha").
	Name []string

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
func (tk *toolkit) List(ctx context.Context, req ListRequest) (*ListResponse, error) {
	var resp ListResponse
	matcher := newNameMatcher(req.Name)
	namespaces := newNamespaceMatcher(req.Namespaces)

	// Validate the type field upfront.
	switch req.Type {
	case ListTypeTools, ListTypePrompts, ListTypeResources:
		// valid
	default:
		return nil, schema.ErrBadParameter.Withf("unsupported list type %q (want %q, %q, or %q)", req.Type, ListTypeTools, ListTypePrompts, ListTypeResources)
	}

	// Collect builtin items and connector candidates under the read lock.
	tk.mu.RLock()
	if namespaces.match(BuiltinNamespace) {
		switch req.Type {
		case ListTypeTools:
			resp.Tools = slices.Collect(filterSeq(maps.Values(tk.tools), func(t llm.Tool) bool {
				return matcher.matchQualified(t.Name(), bareToolName(t))
			}))
		case ListTypePrompts:
			resp.Prompts = slices.Collect(filterSeq(maps.Values(tk.prompts), func(p llm.Prompt) bool {
				return matcher.matchQualified(p.Name(), barePromptName(p))
			}))
		case ListTypeResources:
			resp.Resources = slices.Collect(filterSeq(maps.Values(tk.resources), func(r llm.Resource) bool {
				return matcher.matchExact(r.URI())
			}))
		}
	}
	var candidates []*connector
	for _, c := range tk.namespace {
		if namespaces.match(c.namespace) {
			candidates = append(candidates, c)
		}
	}
	tk.mu.RUnlock()

	// Query each connector in parallel and merge results.
	if len(candidates) > 0 {
		var mu sync.Mutex
		var eg errgroup.Group
		for _, c := range candidates {
			c := c
			eg.Go(func() error {
				switch req.Type {
				case ListTypeTools:
					tools, err := c.ListTools(ctx)
					if err != nil {
						return err
					}
					var wrapped []llm.Tool
					for _, t := range tools {
						w := tool.WithNamespace(c.namespace, t)
						if matcher.matchQualified(w.Name(), t.Name()) {
							wrapped = append(wrapped, w)
						}
					}
					mu.Lock()
					resp.Tools = append(resp.Tools, wrapped...)
					mu.Unlock()
				case ListTypePrompts:
					prompts, err := c.ListPrompts(ctx)
					if err != nil {
						return err
					}
					var wrapped []llm.Prompt
					for _, p := range prompts {
						w := prompt.WithNamespace(c.namespace, p)
						if matcher.matchQualified(w.Name(), p.Name()) {
							wrapped = append(wrapped, w)
						}
					}
					mu.Lock()
					resp.Prompts = append(resp.Prompts, wrapped...)
					mu.Unlock()
				case ListTypeResources:
					resources, err := c.ListResources(ctx)
					if err != nil {
						return err
					}
					var wrapped []llm.Resource
					for _, r := range resources {
						w := resource.WithNamespace(c.namespace, r)
						if matcher.matchExact(w.URI()) {
							wrapped = append(wrapped, w)
						}
					}
					mu.Lock()
					resp.Resources = append(resp.Resources, wrapped...)
					mu.Unlock()
				}
				return nil
			})
		}
		if err := eg.Wait(); err != nil {
			return nil, err
		}
	}

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
	if resp.Limit != nil && *resp.Limit > 0 {
		resp.Tools = paginateSlice(resp.Tools, req.Offset, resp.Limit)
		resp.Prompts = paginateSlice(resp.Prompts, req.Offset, resp.Limit)
		resp.Resources = paginateSlice(resp.Resources, req.Offset, resp.Limit)
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

type nameMatcher struct {
	all   bool
	exact map[string]struct{}
	bare  map[string]struct{}
}

type namespaceMatcher struct {
	all     bool
	allowed map[string]struct{}
}

func newNameMatcher(filters []string) nameMatcher {
	matcher := nameMatcher{
		all:   true,
		exact: make(map[string]struct{}),
		bare:  make(map[string]struct{}),
	}
	for _, filter := range filters {
		if filter == "" {
			continue
		}
		matcher.all = false
		matcher.exact[filter] = struct{}{}
		if !strings.Contains(filter, ".") {
			matcher.bare[filter] = struct{}{}
		}
	}
	return matcher
}

func newNamespaceMatcher(filters []string) namespaceMatcher {
	matcher := namespaceMatcher{
		all:     true,
		allowed: make(map[string]struct{}),
	}
	for _, filter := range filters {
		if filter == "" {
			continue
		}
		matcher.all = false
		matcher.allowed[filter] = struct{}{}
	}
	return matcher
}

func (matcher namespaceMatcher) match(namespace string) bool {
	if matcher.all {
		return true
	}
	_, ok := matcher.allowed[namespace]
	return ok
}

func (matcher nameMatcher) matchExact(name string) bool {
	if matcher.all {
		return true
	}
	_, ok := matcher.exact[name]
	return ok
}

func (matcher nameMatcher) matchQualified(qualifiedName, bareName string) bool {
	if matcher.all {
		return true
	}
	if _, ok := matcher.exact[qualifiedName]; ok {
		return true
	}
	_, ok := matcher.bare[bareName]
	return ok
}

func bareToolName(tool llm.Tool) string {
	type unwrapper interface{ Unwrap() llm.Tool }
	if wrapped, ok := tool.(unwrapper); ok {
		return wrapped.Unwrap().Name()
	}
	return tool.Name()
}

func barePromptName(prompt llm.Prompt) string {
	type unwrapper interface{ Unwrap() llm.Prompt }
	if wrapped, ok := prompt.(unwrapper); ok {
		return wrapped.Unwrap().Name()
	}
	return prompt.Name()
}
