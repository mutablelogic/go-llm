package toolkit

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	eventschema "github.com/mutablelogic/go-llm/broadcaster/schema"
	schema "github.com/mutablelogic/go-llm/toolkit/schema"
	workspace "github.com/mutablelogic/go-llm/workspace"
	workspaceschema "github.com/mutablelogic/go-llm/workspace/schema"
	jsonschema "github.com/mutablelogic/go-server/pkg/jsonschema"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// workspaceNamespace is the namespace of the tools which list, and read,
// the toolkit's workspaces
type workspaceNamespace struct {
	toolkit *Toolkit
	tools   []llm.Tool
}

// listWorkspace lists the workspaces, or a workspace's folders and
// resources
type listWorkspace struct {
	Base
	toolkit *Toolkit
}

// readWorkspace reads a resource of a workspace
type readWorkspace struct {
	Base
	toolkit *Toolkit
}

// ListWorkspaceRequest is the input of the workspace's list tool
type ListWorkspaceRequest struct {
	Workspace string `json:"workspace,omitempty" help:"The workspace whose folders and resources are listed, or empty to list the workspaces"`
	workspaceschema.ListRequest
}

// ReadWorkspaceRequest is the input of the workspace's read tool
type ReadWorkspaceRequest struct {
	Workspace string `json:"workspace,omitempty" help:"The workspace the resource is in, or empty to find it in any workspace"`
	URI       string `json:"uri" help:"The resource's URI, as it's listed"`
}

// WorkspaceMeta is a workspace, as the list tool lists it: its name, and
// its namespace's description
type WorkspaceMeta struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

var _ Namespace = (*workspaceNamespace)(nil)

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

// WorkspaceNamespace is the name of the namespace of the tools which list,
// and read, the toolkit's workspaces, which it has with WithWorkspaces
const WorkspaceNamespace = "workspace"

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Workspaces returns the toolkit's registry of workspaces, or nil, when it
// has none
func (self *Toolkit) Workspaces() *workspace.Registry {
	return self.workspaces
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// addWorkspace adds the namespace to the toolkit's workspaces, by its name,
// when it's a workspace, and the toolkit has them. Returns ErrConflict when
// another workspace has its name.
func (self *Toolkit) addWorkspace(ns Namespace) error {
	ws, ok := ns.(workspace.Workspace)
	if !ok || self.workspaces == nil {
		return nil
	}
	return self.workspaces.Add(ns.Name(), ws)
}

// removeWorkspace removes the namespace from the toolkit's workspaces, when
// it's one of them, rather than a workspace which replaced it, by its name
func (self *Toolkit) removeWorkspace(ns Namespace) {
	if ws, ok := ns.(workspace.Workspace); ok && self.workspaces != nil {
		self.workspaces.RemoveIf(ns.Name(), ws)
	}
}

// withWorkspaces returns the context, whose resolve function reads the
// resources a tool is given from the toolkit's workspaces, when it can't,
// or when there's none, so a tool reads a resource it was given from a
// workspace's listing
func (self *Toolkit) withWorkspaces(ctx context.Context) context.Context {
	if self.workspaces == nil {
		return ctx
	}
	next, _ := ctx.Value(resolveKey{}).(ResolveFunc)
	return WithResolver(ctx, func(ctx context.Context, uris ...string) ([]llm.Resource, error) {
		if next != nil {
			resources, err := next(ctx, uris...)
			if err == nil || !fallback(err) {
				return resources, err
			}
		}
		result := make([]llm.Resource, 0, len(uris))
		for _, uri := range uris {
			resource, err := self.resolveWorkspace(ctx, next, uri)
			if err != nil {
				return nil, err
			}
			result = append(result, resource)
		}
		return result, nil
	})
}

// resolveWorkspace returns the resource with the URI, with the resolve function,
// when there's one, which has it, or otherwise from a workspace
func (self *Toolkit) resolveWorkspace(ctx context.Context, next ResolveFunc, uri string) (llm.Resource, error) {
	if next != nil {
		resources, err := next(ctx, uri)
		if err == nil && len(resources) == 1 {
			return resources[0], nil
		} else if err != nil && !fallback(err) {
			// It failed, rather than not having it, which is said
			return nil, err
		}
	}
	return self.workspaces.Resolve(ctx, uri)
}

///////////////////////////////////////////////////////////////////////////////
// NAMESPACE

// newWorkspaceNamespace returns the namespace of the tools which list, and
// read, the toolkit's workspaces
func newWorkspaceNamespace(tk *Toolkit) *workspaceNamespace {
	return &workspaceNamespace{toolkit: tk, tools: []llm.Tool{&listWorkspace{toolkit: tk}, &readWorkspace{toolkit: tk}}}
}

func (*workspaceNamespace) Name() string { return WorkspaceNamespace }
func (*workspaceNamespace) Type() string { return schema.NamespaceBuiltin }
func (*workspaceNamespace) Description() string {
	return "Workspaces: folders and resources, such as notes and files"
}
func (*workspaceNamespace) SystemPrompt() string {
	return "Workspaces have folders and resources, such as notes and files. List the workspaces, then a workspace's folders, and read a resource by its URI, as it's listed."
}
func (n *workspaceNamespace) Tools() []llm.Tool { return n.tools }
func (n *workspaceNamespace) Watch(ctx context.Context, ch chan<- eventschema.Event) error {
	return WatchTools(ctx, ch, n.tools)
}

///////////////////////////////////////////////////////////////////////////////
// LIST

func (*listWorkspace) Name() string { return "list" }
func (*listWorkspace) Description() string {
	return "List the workspaces, without a workspace, or a workspace's folders and resources, in a folder, or under it, a page at a time"
}
func (*listWorkspace) InputSchema() *jsonschema.Schema {
	return jsonschema.MustFor[ListWorkspaceRequest]()
}

// Run lists the workspaces, with their namespaces' descriptions, or the
// workspace's folders and resources
func (t *listWorkspace) Run(ctx context.Context, input json.RawMessage) (any, error) {
	var req ListWorkspaceRequest
	if err := unmarshalInput(input, &req); err != nil {
		return nil, err
	}
	registry := t.toolkit.workspaces
	if strings.TrimSpace(req.Workspace) == "" {
		result := []WorkspaceMeta{}
		for name := range registry.All() {
			meta := WorkspaceMeta{Name: name}
			t.toolkit.mu.RLock()
			if ns, exists := t.toolkit.namespaces[name]; exists {
				meta.Description = ns.Description()
			}
			t.toolkit.mu.RUnlock()
			result = append(result, meta)
		}
		return result, nil
	}
	var result *workspaceschema.ListResponse
	err := registry.With(req.Workspace, func(ws workspace.Workspace) (err error) {
		result, err = ws.List(ctx, req.ListRequest)
		return err
	})
	return result, err
}

///////////////////////////////////////////////////////////////////////////////
// READ

func (*readWorkspace) Name() string { return "read" }
func (*readWorkspace) Description() string {
	return "Read a resource of a workspace, such as a note or file, by its URI, as it's listed"
}
func (*readWorkspace) InputSchema() *jsonschema.Schema {
	return jsonschema.MustFor[ReadWorkspaceRequest]()
}

// Run reads the resource, from its workspace, or any which has it
func (t *readWorkspace) Run(ctx context.Context, input json.RawMessage) (any, error) {
	var req ReadWorkspaceRequest
	if err := unmarshalInput(input, &req); err != nil {
		return nil, err
	} else if strings.TrimSpace(req.URI) == "" {
		return nil, llm.ErrBadParameter.With("missing uri")
	}
	if strings.TrimSpace(req.Workspace) == "" {
		return t.toolkit.workspaces.Resolve(ctx, req.URI)
	}
	return t.toolkit.workspaces.Read(ctx, req.Workspace, req.URI)
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// fallback returns true when the resolve function's error is that it
// doesn't have a resource, rather than that it failed, so the workspaces
// are tried
func fallback(err error) bool {
	return errors.Is(err, llm.ErrNotFound) || errors.Is(err, llm.ErrNotImplemented) || errors.Is(err, llm.ErrBadParameter)
}

// unmarshalInput decodes the tool's input, which may be empty
func unmarshalInput(input json.RawMessage, v any) error {
	if len(input) == 0 {
		return nil
	}
	if err := json.Unmarshal(input, v); err != nil {
		return llm.ErrBadParameter.Withf("input: %v", err)
	}
	return nil
}
