package toolkit

import (
	"context"
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
		tool.OutputToolName,
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
// or URI#namespace. Returns (nil, nil) if nothing matches.
func (tk *toolkit) Lookup(_ context.Context, key string) (any, error) {
	// URI or URI#namespace: if the key parses as a URI, only resources can match.
	if u, namespace, ok := parseURI(key); ok {
		return tk.lookupURI(namespace, u.String())
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
		tool, err = tk.lookupTool(namespace, name)
		return err
	})
	eg.Go(func() error {
		var err error
		prompt, err = tk.lookupPrompt(namespace, name)
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
// Only the builtin namespace (or empty) is supported.
func (tk *toolkit) lookupTool(namespace, name string) (llm.Tool, error) {
	if namespace != "" && namespace != NamespaceBuiltin {
		return nil, nil
	}
	// The output tool is always available by its reserved name.
	if name == tool.OutputToolName {
		return tool.WithNamespace(NamespaceBuiltin, tool.NewOutputTool(nil)), nil
	}
	tk.mu.RLock()
	defer tk.mu.RUnlock()
	return tk.tools[name], nil
}

// lookupPrompt returns the prompt registered under name in the given namespace.
// Only the builtin namespace (or empty) is supported.
func (tk *toolkit) lookupPrompt(namespace, name string) (llm.Prompt, error) {
	if namespace != "" && namespace != NamespaceBuiltin {
		return nil, nil
	}
	tk.mu.RLock()
	defer tk.mu.RUnlock()
	return tk.prompts[name], nil
}

// lookupURI returns the resource registered under the given namespace and
// bare URI (fragment already stripped). Only the builtin namespace (or empty)
// is supported; any other namespace returns nil.
func (tk *toolkit) lookupURI(namespace, uri string) (llm.Resource, error) {
	if namespace != "" && namespace != NamespaceBuiltin {
		return nil, nil
	}
	tk.mu.RLock()
	defer tk.mu.RUnlock()
	return tk.resources[uri], nil
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
