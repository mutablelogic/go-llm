package toolkit

import (
	"cmp"
	"context"
	"net/url"
	"slices"
	"sync"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	"github.com/mutablelogic/go-server/pkg/types"
	trace "go.opentelemetry.io/otel/trace"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type toolkit struct {
	tracer trace.Tracer

	// Builtin tools, prompts, and resources are stored in maps for efficient lookup.
	mu        sync.RWMutex
	tools     map[string]llm.Tool
	prompts   map[string]llm.Prompt
	resources map[string]llm.Resource

	// Builtin connectors are stored by URL
	//connector map[string]*conn

	// handler receives callbacks for connector lifecycle events, prompt execution, etc
	handler ToolkitHandler
}

var _ Toolkit = (*toolkit)(nil)

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	NamespaceBuiltin = "builtin"
	NamespaceUser    = "user"
)

var (
	ReservedNames = []string{
		"submit_output",
	}
	ReservedNamespaces = []string{
		NamespaceBuiltin,
		NamespaceUser,
	}
)

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// NewToolkit creates a new Toolkit with the given options.
func New(opts ...Option) (*toolkit, error) {
	toolkit := new(toolkit)
	toolkit.tools = make(map[string]llm.Tool)
	toolkit.prompts = make(map[string]llm.Prompt)
	toolkit.resources = make(map[string]llm.Resource)

	// Apply options
	for _, opt := range opts {
		if err := opt(toolkit); err != nil {
			return nil, err
		}
	}

	// Return success
	return toolkit, nil
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// AddTool registers one or more builtin tools.
func (tk *toolkit) AddTool(tools ...llm.Tool) error {
	tk.mu.Lock()
	defer tk.mu.Unlock()

	seen := make(map[string]struct{}, len(tools))
	for _, t := range tools {
		if t == nil {
			continue
		}
		if name := t.Name(); !types.IsIdentifier(name) {
			return llm.ErrBadParameter.Withf("invalid tool name: %q", name)
		} else if slices.Contains(ReservedNames, name) {
			return llm.ErrBadParameter.Withf("reserved tool name: %q", name)
		} else if _, exists := tk.tools[name]; exists {
			return llm.ErrBadParameter.Withf("duplicate tool name: %q", name)
		} else if _, exists := seen[name]; exists {
			return llm.ErrBadParameter.Withf("duplicate tool name: %q", name)
		} else {
			seen[name] = struct{}{}
		}
	}
	for _, t := range tools {
		if t != nil {
			tk.tools[t.Name()] = t
		}
	}

	// Return success
	return nil
}

// AddPrompt registers one or more builtin prompts.
func (tk *toolkit) AddPrompt(prompts ...llm.Prompt) error {
	tk.mu.Lock()
	defer tk.mu.Unlock()

	seen := make(map[string]struct{}, len(prompts))
	for _, p := range prompts {
		if p == nil {
			continue
		}
		if name := p.Name(); !types.IsIdentifier(name) {
			return llm.ErrBadParameter.Withf("invalid prompt name: %q", name)
		} else if slices.Contains(ReservedNames, name) {
			return llm.ErrBadParameter.Withf("reserved prompt name: %q", name)
		} else if _, exists := tk.prompts[name]; exists {
			return llm.ErrBadParameter.Withf("duplicate prompt name: %q", name)
		} else if _, exists := seen[name]; exists {
			return llm.ErrBadParameter.Withf("duplicate prompt name: %q", name)
		} else {
			seen[name] = struct{}{}
		}
	}
	for _, p := range prompts {
		if p != nil {
			tk.prompts[p.Name()] = p
		}
	}

	// Return success
	return nil
}

// AddResource registers one or more builtin resources.
func (tk *toolkit) AddResource(resources ...llm.Resource) error {
	tk.mu.Lock()
	defer tk.mu.Unlock()

	// parseURI parses, validates and canonicalises a resource URI:
	// the scheme must be a valid identifier, the URI must have a non-empty
	// opaque part, host, or path, and any fragment is stripped.
	parseURI := func(r llm.Resource) (string, error) {
		raw := r.URI()
		u, err := url.Parse(raw)
		if err != nil || !types.IsIdentifier(u.Scheme) || (u.Opaque == "" && u.Host == "" && u.Path == "") {
			return "", llm.ErrBadParameter.Withf("invalid resource URI: %q", raw)
		}
		u.Fragment = ""
		return u.String(), nil
	}

	seen := make(map[string]struct{}, len(resources))
	for _, r := range resources {
		if r == nil {
			continue
		}
		uri, err := parseURI(r)
		if err != nil {
			return err
		} else if _, exists := tk.resources[uri]; exists {
			return llm.ErrBadParameter.Withf("duplicate resource URI: %q", uri)
		} else if _, exists := seen[uri]; exists {
			return llm.ErrBadParameter.Withf("duplicate resource URI: %q", uri)
		}
		seen[uri] = struct{}{}
	}
	for _, r := range resources {
		if r != nil {
			uri, _ := parseURI(r)
			tk.resources[uri] = r
		}
	}

	// Return success
	return nil
}

// List returns tools, prompts, and resources matching the request.
// Only the "builtin" namespace is currently supported.
func (tk *toolkit) List(_ context.Context, req ListRequest) (*ListResponse, error) {
	tk.mu.RLock()
	defer tk.mu.RUnlock()

	// Reject unsupported namespaces (user and connectors not yet implemented).
	if req.Namespace != "" && req.Namespace != NamespaceBuiltin {
		return nil, llm.ErrNotImplemented
	}

	// When no type filter is set, include all types.
	if !req.Tools && !req.Prompts && !req.Resources {
		req.Tools, req.Prompts, req.Resources = true, true, true
	}

	resp := &ListResponse{
		Offset: req.Offset,
	}

	if req.Tools {
		for _, t := range tk.tools {
			resp.Tools = append(resp.Tools, t)
		}
		slices.SortFunc(resp.Tools, func(a, b llm.Tool) int {
			return cmp.Compare(a.Name(), b.Name())
		})
	}
	if req.Prompts {
		for _, p := range tk.prompts {
			resp.Prompts = append(resp.Prompts, p)
		}
		slices.SortFunc(resp.Prompts, func(a, b llm.Prompt) int {
			return cmp.Compare(a.Name(), b.Name())
		})
	}
	if req.Resources {
		for _, r := range tk.resources {
			resp.Resources = append(resp.Resources, r)
		}
		slices.SortFunc(resp.Resources, func(a, b llm.Resource) int {
			return cmp.Compare(a.URI(), b.URI())
		})
	}

	// Count total items before pagination.
	total := uint(len(resp.Tools) + len(resp.Prompts) + len(resp.Resources))
	resp.Count = total

	// Apply offset and limit across the flat item count.
	// For now, pagination is advisory — slices are already small for builtins.
	if req.Limit != nil {
		resp.Limit = *req.Limit
	}

	// Return success
	return resp, nil
}

// RemoveBuiltin removes a previously registered builtin tool by name,
// prompt by name, or resource by URI.
// Returns an error if the identifier matches zero or more than one item.
func (tk *toolkit) RemoveBuiltin(string) error {
	return llm.ErrNotImplemented
}

// AddConnector registers a remote MCP server. The namespace is inferred from
// the server (e.g. the hostname or last path segment of the URL). Safe to call
// before or while Run is active; the connector starts immediately if Run is
// already running.
func (tk *toolkit) AddConnector(string) error {
	return llm.ErrNotImplemented
}

// AddConnectorNS registers a remote MCP server under an explicit namespace.
// Safe to call before or while Run is active; the connector starts immediately
// if Run is already running.
func (tk *toolkit) AddConnectorNS(namespace, url string) error {
	return llm.ErrNotImplemented
}

// RemoveConnector removes a connector by URL. Safe to call before or
// while Run is active; the connector is stopped immediately if running.
func (tk *toolkit) RemoveConnector(string) error {
	return llm.ErrNotImplemented
}

// Run starts all queued connectors and blocks until ctx is cancelled.
// It closes the toolkit and waits for all connectors to finish on return.
func (tk *toolkit) Run(context.Context) error {
	return llm.ErrNotImplemented
}

// Lookup finds a tool, prompt, or resource by name, namespace.name, URI,
// or URI#namespace. Returns nil if nothing matches.
func (tk *toolkit) Lookup(context.Context, string) any {
	return llm.ErrNotImplemented
}

// Call executes a tool or prompt, passing optional resource arguments.
// For tools, resources are made available via the session context.
// For prompts, the first resource supplies template variables and any
// remaining resources are attached to the generated message.
func (tk *toolkit) Call(context.Context, any, ...llm.Resource) (llm.Resource, error) {
	return nil, llm.ErrNotImplemented
}
