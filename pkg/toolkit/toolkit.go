package toolkit

import (
	"context"
	"log/slog"
	"net/url"
	"slices"
	"strings"
	"sync"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	prompt "github.com/mutablelogic/go-llm/pkg/toolkit/prompt"
	resource "github.com/mutablelogic/go-llm/pkg/toolkit/resource"
	tool "github.com/mutablelogic/go-llm/pkg/toolkit/tool"
	types "github.com/mutablelogic/go-server/pkg/types"
	trace "go.opentelemetry.io/otel/trace"
	errgroup "golang.org/x/sync/errgroup"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type toolkit struct {
	mu     sync.RWMutex
	tracer trace.Tracer
	logger *slog.Logger

	// Builtin tools, prompts, and resources are stored in maps for efficient lookup.
	tools     map[string]llm.Tool
	prompts   map[string]llm.Prompt
	resources map[string]llm.Resource

	// Connectors by URL and namespace
	connectors map[string]*connector
	namespace  map[string]*connector

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
		tool.OutputToolName,
	}
	ReservedNamespaces = []string{
		NamespaceBuiltin,
		NamespaceUser,
	}
)

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// New creates a new Toolkit with the given options.
func New(opts ...Option) (*toolkit, error) {
	toolkit := new(toolkit)

	// Set default logger
	toolkit.logger = slog.Default()

	// Builtins
	toolkit.tools = make(map[string]llm.Tool)
	toolkit.prompts = make(map[string]llm.Prompt)
	toolkit.resources = make(map[string]llm.Resource)
	toolkit.connectors = make(map[string]*connector)
	toolkit.namespace = make(map[string]*connector)

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
			tk.tools[t.Name()] = tool.WithNamespace(NamespaceBuiltin, t)
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
			tk.prompts[p.Name()] = prompt.WithNamespace(NamespaceBuiltin, p)
		}
	}

	// Return success
	return nil
}

// AddResource registers one or more builtin resources.
func (tk *toolkit) AddResource(resources ...llm.Resource) error {
	tk.mu.Lock()
	defer tk.mu.Unlock()

	seen := make(map[string]struct{}, len(resources))
	for _, r := range resources {
		if r == nil {
			continue
		}
		u, _, ok := parseURI(r.URI())
		if !ok {
			return llm.ErrBadParameter.Withf("invalid resource URI: %q", r.URI())
		}
		uri := u.String()
		if _, exists := tk.resources[uri]; exists {
			return llm.ErrBadParameter.Withf("duplicate resource URI: %q", uri)
		} else if _, exists := seen[uri]; exists {
			return llm.ErrBadParameter.Withf("duplicate resource URI: %q", uri)
		}
		seen[uri] = struct{}{}
	}
	for _, r := range resources {
		if r != nil {
			u, _, _ := parseURI(r.URI())
			tk.resources[u.String()] = resource.WithNamespace(NamespaceBuiltin, r)
		}
	}

	// Return success
	return nil
}

// RemoveBuiltin removes a previously registered builtin tool by name,
// prompt by name, or resource by URI. Tools are checked before prompts.
// Returns llm.ErrNotFound if no match exists.
func (tk *toolkit) RemoveBuiltin(key string) error {
	tk.mu.Lock()
	defer tk.mu.Unlock()
	if _, ok := tk.tools[key]; ok {
		delete(tk.tools, key)
		return nil
	}
	if _, ok := tk.prompts[key]; ok {
		delete(tk.prompts, key)
		return nil
	}
	if u, _, ok := parseURI(key); ok {
		uri := u.String()
		if _, ok := tk.resources[uri]; ok {
			delete(tk.resources, uri)
			return nil
		}
	}
	return llm.ErrNotFound.Withf("%q", key)
}

// Lookup finds a tool, prompt, or resource by name, namespace.name, URI,
// or URI#namespace. Tools take precedence over prompts when both share a name.
// Returns llm.ErrNotFound if nothing matches.
func (tk *toolkit) Lookup(ctx context.Context, key string) (any, error) {
	// URI or URI#namespace: if the key parses as a URI, only resources can match.
	if u, namespace, ok := parseURI(key); ok {
		return tk.lookupResource(ctx, namespace, u.String())
	}

	// Parse optional namespace prefix and bare name.
	var namespace, name string
	if ns, n, ok := strings.Cut(key, "."); ok {
		namespace, name = ns, n
	} else {
		name = key
	}
	if !types.IsIdentifier(name) || (namespace != "" && !types.IsIdentifier(namespace)) {
		return nil, llm.ErrBadParameter.Withf("invalid key: %q", key)
	}

	// Search tools and prompts concurrently; tools take precedence over prompts.
	var tool llm.Tool
	var prompt llm.Prompt
	var eg errgroup.Group
	eg.Go(func() error {
		var err error
		tool, err = tk.lookupTool(ctx, namespace, name)
		return err
	})
	eg.Go(func() error {
		var err error
		prompt, err = tk.lookupPrompt(ctx, namespace, name)
		return err
	})
	if err := eg.Wait(); err != nil {
		return nil, err
	}

	// Tools take precedence over prompts when both match the same name.
	if tool != nil {
		return tool, nil
	}
	if prompt != nil {
		return prompt, nil
	}

	// No match found.
	return nil, llm.ErrNotFound.Withf("%q", key)
}

// lookupTool returns the tool registered under name in the given namespace.
// When namespace is empty, builtins are checked first then all connected
// connectors are searched. When namespace is "builtin", only builtins are
// searched. Otherwise the named connector namespace is searched via ListTools.
func (tk *toolkit) lookupTool(ctx context.Context, namespace, name string) (llm.Tool, error) {
	// The output tool is always available by its reserved name in the builtin (or empty) namespace.
	if name == tool.OutputToolName && (namespace == "" || namespace == NamespaceBuiltin) {
		return tool.WithNamespace(NamespaceBuiltin, tool.NewOutputTool(nil)), nil
	}
	// Builtin namespace (or no namespace): check the in-process tools map first.
	if namespace == "" || namespace == NamespaceBuiltin {
		tk.mu.RLock()
		t := tk.tools[name]
		tk.mu.RUnlock()
		if t != nil || namespace == NamespaceBuiltin {
			return t, nil
		}
		// namespace == "": fall through to search all connected connectors.
	}
	// Collect the connectors to search: either one specific namespace or all.
	tk.mu.RLock()
	var candidates []*connector
	if namespace != "" {
		if c := tk.namespace[namespace]; c != nil {
			candidates = []*connector{c}
		}
	} else {
		for _, c := range tk.namespace {
			candidates = append(candidates, c)
		}
	}
	tk.mu.RUnlock()

	for _, c := range candidates {
		tools, err := c.ListTools(ctx)
		if err != nil {
			return nil, err
		}
		for _, t := range tools {
			if t.Name() == name {
				return tool.WithNamespace(c.namespace, t), nil
			}
		}
	}
	return nil, nil
}

// lookupPrompt returns the prompt registered under name in the given namespace.
// When namespace is empty, builtins are checked first then all connected
// connectors are searched. When namespace is "builtin", only builtins are
// searched. Otherwise the named connector namespace is searched via ListPrompts.
func (tk *toolkit) lookupPrompt(ctx context.Context, namespace, name string) (llm.Prompt, error) {
	// Builtin namespace (or no namespace): check the in-process prompts map first.
	if namespace == "" || namespace == NamespaceBuiltin {
		tk.mu.RLock()
		p := tk.prompts[name]
		tk.mu.RUnlock()
		if p != nil || namespace == NamespaceBuiltin {
			return p, nil
		}
		// namespace == "": fall through to search all connected connectors.
	}
	// Collect the connectors to search: either one specific namespace or all.
	tk.mu.RLock()
	var candidates []*connector
	if namespace != "" {
		if c := tk.namespace[namespace]; c != nil {
			candidates = []*connector{c}
		}
	} else {
		for _, c := range tk.namespace {
			candidates = append(candidates, c)
		}
	}
	tk.mu.RUnlock()

	for _, c := range candidates {
		prompts, err := c.ListPrompts(ctx)
		if err != nil {
			return nil, err
		}
		for _, p := range prompts {
			if p.Name() == name {
				return prompt.WithNamespace(c.namespace, p), nil
			}
		}
	}
	return nil, nil
}

// lookupResource returns the resource registered under the given namespace and
// bare URI (fragment already stripped). When namespace is empty, builtins are
// checked first then all connected connectors are searched. When namespace is
// "builtin", only builtins are searched. Otherwise the named connector
// namespace is searched via ListResources.
// Returns llm.ErrNotFound when no resource matches.
func (tk *toolkit) lookupResource(ctx context.Context, namespace, uri string) (llm.Resource, error) {
	// Builtin namespace (or no namespace): check the in-process resources map first.
	if namespace == "" || namespace == NamespaceBuiltin {
		tk.mu.RLock()
		r := tk.resources[uri]
		tk.mu.RUnlock()
		if r != nil {
			return r, nil
		}
		if namespace == NamespaceBuiltin {
			return nil, llm.ErrNotFound.Withf("%q", uri)
		}
		// namespace == "": fall through to search all connected connectors.
	}
	// Collect the connectors to search: either one specific namespace or all.
	tk.mu.RLock()
	var candidates []*connector
	if namespace != "" {
		if c := tk.namespace[namespace]; c != nil {
			candidates = []*connector{c}
		}
	} else {
		for _, c := range tk.namespace {
			candidates = append(candidates, c)
		}
	}
	tk.mu.RUnlock()

	for _, c := range candidates {
		resources, err := c.ListResources(ctx)
		if err != nil {
			return nil, err
		}
		for _, r := range resources {
			if r.URI() == uri {
				return resource.WithNamespace(c.namespace, r), nil
			}
		}
	}
	return nil, llm.ErrNotFound.Withf("%q", uri)
}

// parseURI parses raw into a *url.URL with the fragment stripped, and returns
// the fragment separately. The scheme must be a valid identifier and the URI
// must have a non-empty opaque part, host, or path. Returns (nil, "", false)
// on any failure.
func parseURI(raw string) (*url.URL, string, bool) {
	u, err := url.Parse(raw)
	if err != nil || !types.IsIdentifier(u.Scheme) || (u.Opaque == "" && u.Host == "" && u.Path == "") {
		return nil, "", false
	}
	fragment := u.Fragment
	u.Fragment = ""
	return u, fragment, true
}
