package toolkit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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
	URI    string `json:"uri" help:"The resource's URI, as it's listed, resource://<workspace>/<uri>"`
	Format string `json:"format,omitempty" enum:"raw,markdown" help:"raw, for the resource as it is, up to 10 MiB, which is the default, or markdown, for its text, as a Markdown document, a part at a time, such as of a PDF, or an Office document, whose YAML front matter has its metadata, and the offset of the next part"`
	Offset uint64 `json:"offset,omitempty" help:"With markdown, the byte of its Markdown the part starts at, which is a part's next"`
	Limit  uint64 `json:"limit,omitempty" help:"With markdown, the most bytes of its Markdown which are read"`
}

// WorkspaceMeta is a workspace, as the list tool lists it: its name, and
// its namespace's description
type WorkspaceMeta struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// readResource is a resource whose content has been read
type readResource struct {
	llm.Resource
	data []byte
}

var _ Namespace = (*workspaceNamespace)(nil)
var _ llm.Sized = (*readResource)(nil)

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

// WorkspaceNamespace is the name of the namespace of the tools which list,
// and read, the toolkit's workspaces, which it has with WithWorkspaces
const WorkspaceNamespace = "workspace"

// MaxRawSize is the most bytes of a resource the read tool returns as it
// is, which a model is sent whole. A larger one is read with the markdown
// format, a part at a time.
const MaxRawSize = 10 * 1024 * 1024

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
	return registry.ListResources(ctx, req.Workspace, req.ListRequest)
}

///////////////////////////////////////////////////////////////////////////////
// READ

func (*readWorkspace) Name() string { return "read" }
func (*readWorkspace) Description() string {
	return "Read a resource of a workspace, such as a note or file, by its URI, as it's listed: as it is, or, with the markdown format, its text, as Markdown, a part at a time"
}
func (*readWorkspace) InputSchema() *jsonschema.Schema {
	return jsonschema.MustFor[ReadWorkspaceRequest]()
}

// Run reads the resource, from the workspace its URI says, or, for its own
// URI, whichever has it
func (t *readWorkspace) Run(ctx context.Context, input json.RawMessage) (any, error) {
	var req ReadWorkspaceRequest
	if err := unmarshalInput(input, &req); err != nil {
		return nil, err
	} else if strings.TrimSpace(req.URI) == "" {
		return nil, llm.ErrBadParameter.With("missing uri")
	}
	resource, err := t.toolkit.workspaces.Resolve(ctx, req.URI)
	if err != nil {
		return nil, err
	}
	switch format := strings.ToLower(strings.TrimSpace(req.Format)); format {
	case "", "raw":
		return readRaw(ctx, req.URI, resource)
	case "markdown":
		part, err := workspace.ReadMarkdown(ctx, resource, req.Offset, req.Limit)
		if err != nil {
			return nil, err
		}
		return part.Document(), nil
	default:
		return nil, llm.ErrBadParameter.Withf("unknown format %q, which isn't raw or markdown", req.Format)
	}
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// readRaw returns the resource, as it is, when it's no larger than
// MaxRawSize, which, when its size isn't known, is read, to find out.
// Returns ErrBadParameter when it's larger, which says to read it with the
// markdown format.
func readRaw(ctx context.Context, uri string, resource llm.Resource) (llm.Resource, error) {
	tooLarge := func(size string) error {
		return llm.ErrBadParameter.Withf("%s is %s, which is more than the %d bytes which are read raw, so read it with the markdown format, a part at a time", uri, size, MaxRawSize)
	}
	if size, ok := llm.ContentLength(resource); ok {
		if size > MaxRawSize {
			return nil, tooLarge(fmt.Sprintf("%d bytes", size))
		}
		return resource, nil
	}

	// Its size isn't known, so it's read, as far as is more than it can be
	read, err := resource.Read(ctx)
	if err != nil {
		return nil, err
	}
	var data []byte
	if err := read(ctx, func(r io.Reader) error {
		data, err = io.ReadAll(io.LimitReader(r, MaxRawSize+1))
		return err
	}); err != nil {
		return nil, err
	} else if len(data) > MaxRawSize {
		return nil, tooLarge("larger")
	}
	return &readResource{Resource: resource, data: data}, nil
}

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

///////////////////////////////////////////////////////////////////////////////
// READ RESOURCE

// ContentLength returns the length of its content, as it was read
func (r *readResource) ContentLength() int64 { return int64(len(r.data)) }

// Read returns the function which reads its content, as it was read
func (r *readResource) Read(context.Context) (llm.ReadFunc, error) {
	return llm.ReadBytes(r.data), nil
}
