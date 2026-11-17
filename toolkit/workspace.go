package toolkit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"strings"
	"unicode/utf8"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/toolkit/schema"
	workspace "github.com/mutablelogic/go-llm/workspace"
	workspaceschema "github.com/mutablelogic/go-llm/workspace/schema"
	eventschema "github.com/mutablelogic/go-server/broadcaster/schema"
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
	Format string `json:"format,omitempty" enum:"raw,markdown" help:"Without it, as its type is: an image, audio, or video, as raw, and anything else as markdown, or raw, when it has none. raw, for its content as it is: text as its text, a part at a time, and anything else, such as an image, or audio, up to 10 MiB, as a file, whose link other tools are given, such as llm:transcribe. markdown, for its text, as a Markdown document, a part at a time, such as of a PDF, or an Office document, whose YAML front matter has its metadata, and the offset of the next part"`
	Offset uint64 `json:"offset,omitempty" help:"As Markdown, or raw text, the byte the part starts at, which is a part's next"`
	Limit  uint64 `json:"limit,omitempty" help:"As Markdown, or raw text, the most bytes which are read"`
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
	return "Workspaces hold folders and resources, such as notes and files. " +
		"workspace:list shows the workspaces, and, with a workspace, its folders and resources: list under a folder by its path, from the listing, not its uri, which is for reading. " +
		"Long listings page: pass a response's next back as cursor, unchanged, with the same path, recursive and type; count, when it's there, is how many there are. type filters, such as inode/directory for folders alone. " +
		"workspace:read reads a resource by its uri: a document's text, as Markdown, a part at a time, whose front matter has its length; pass a part's next back as offset, until a part has none. " +
		"An image, audio or video comes back as a file, whose link you give other tools, such as llm:transcribe, for its speech, rather than reading it. " +
		"There's no search yet: to find something, walk the folders, and don't say you searched. " +
		"When more than one workspace could hold what you want, say which one you read."
}
func (n *workspaceNamespace) Tools() []llm.Tool { return n.tools }
func (n *workspaceNamespace) Watch(ctx context.Context, ch chan<- eventschema.Change) error {
	return WatchTools(ctx, ch, n.tools)
}

///////////////////////////////////////////////////////////////////////////////
// LIST

func (*listWorkspace) Name() string { return "list" }
func (*listWorkspace) Description() string {
	return "List the workspaces, without a workspace, or a workspace's folders and resources, in a folder, by its path, from the listing, or under it, a page at a time, whose next is passed back as cursor"
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
	return "Read a resource of a workspace, such as a note or file, by its URI, as it's listed: its text, as Markdown, a part at a time, or, for an image, audio, or video, as it is"
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
	case "":
		// As its type is: an image, audio, or video, as it is, as a file,
		// whose link other tools are given, and otherwise its text, as
		// Markdown, or, when it has none, as it is
		if media(resource.Type()) {
			return readRaw(ctx, req.URI, resource, req.Offset, req.Limit)
		}
		part, err := workspace.ReadMarkdown(ctx, resource, req.Offset, req.Limit)
		if errors.Is(err, llm.ErrNotImplemented) {
			return readRaw(ctx, req.URI, resource, req.Offset, req.Limit)
		} else if err != nil {
			return nil, err
		}
		return part.Document(), nil
	case "raw":
		return readRaw(ctx, req.URI, resource, req.Offset, req.Limit)
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

// media returns true when the type is an image, audio, or video's, which is
// read as it is, rather than as Markdown
func media(contentType string) bool {
	for _, prefix := range []string{"image/", "audio/", "video/"} {
		if strings.HasPrefix(strings.ToLower(contentType), prefix) {
			return true
		}
	}
	return false
}

// readRaw returns the resource's content, as it is: text, as its text, and
// anything else, such as an image, or audio, as the resource, which the
// model is sent as a file, which it sees, or hears, up to MaxRawSize, which,
// when its size isn't known, is read, to find out. Returns ErrBadParameter
// when it's larger, which says what's read instead.
func readRaw(ctx context.Context, uri string, resource llm.Resource, offset, limit uint64) (any, error) {
	text := textual(resource.Type())
	if text {
		if part, err := readText(ctx, resource, offset, limit); err != nil || part != nil {
			return part, err
		}
	}
	tooLarge := func(size string) error {
		if text {
			return llm.ErrBadParameter.Withf("%s is %s, more than the %d bytes which are read raw, so read it as Markdown, a part at a time, from an offset, with the markdown format", uri, size, MaxRawSize)
		}
		advice := "its metadata is listed by workspace:list"
		if media(resource.Type()) && !strings.HasPrefix(strings.ToLower(resource.Type()), "image/") {
			advice += ", and its speech is transcribed by llm:transcribe"
		}
		return llm.ErrBadParameter.Withf("%s is %s of %s, more than the %d bytes which are sent as a file, and it has no text: %s", uri, size, resource.Type(), MaxRawSize, advice)
	}
	if size, ok := llm.ContentLength(resource); ok && size > MaxRawSize {
		return nil, tooLarge(fmt.Sprintf("%d bytes", size))
	} else if ok && !text {
		return resource, nil
	}

	// Its content, as far as is more than it can be
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
	if text && utf8.Valid(data) {
		return string(data), nil
	}
	return &readResource{Resource: resource, data: data}, nil
}

// readText returns a part of the text, from the byte at the offset, of at
// most the limit, or the default, of Markdown's, in whole characters, as a
// document, whose front matter has its offset, the offset of the next part,
// when there's more, and the length of the text, when it's known, or nil,
// when it isn't text, as it isn't UTF-8. Returns ErrBadParameter when the
// offset is beyond its end.
func readText(ctx context.Context, resource llm.Resource, offset, limit uint64) (any, error) {
	if limit == 0 {
		limit = workspace.DefaultMarkdownLimit
	}
	limit = min(limit, workspace.MaxMarkdownLimit)
	read, err := resource.Read(ctx)
	if err != nil {
		return nil, err
	}

	// The part, and a byte more, so there's more, from the offset, which is
	// sought, or read past
	var data []byte
	if err := read(ctx, func(r io.Reader) error {
		if offset > 0 {
			if seeker, ok := r.(io.Seeker); ok {
				if _, err := seeker.Seek(int64(offset), io.SeekStart); err != nil {
					return err
				}
			} else if _, err := io.CopyN(io.Discard, r, int64(offset)); err != nil && !errors.Is(err, io.EOF) {
				return err
			}
		}
		var err error
		data, err = io.ReadAll(io.LimitReader(r, int64(limit)+1))
		return err
	}); err != nil {
		return nil, err
	}
	size, sized := llm.ContentLength(resource)
	if offset > 0 && ((sized && int64(offset) >= size) || (!sized && len(data) == 0)) {
		return nil, llm.ErrBadParameter.Withf("offset %d is beyond the end of the text, at %d", offset, size)
	}

	// In whole characters
	start := 0
	for offset > 0 && start < len(data) && start < utf8.UTFMax && !utf8.RuneStart(data[start]) {
		start++
	}
	more := uint64(len(data)) > limit
	end := len(data)
	if more {
		end = int(limit)
		for end > start && !utf8.RuneStart(data[end]) {
			end--
		}
	}
	if !utf8.Valid(data[start:end]) {
		return nil, nil
	}
	part := workspaceschema.Markdown{URI: resource.URI(), Type: resource.Type(), Body: string(data[start:end]), Offset: offset + uint64(start)}
	if more {
		next := offset + uint64(end)
		part.Next = &next
	}
	if sized {
		length := uint64(size)
		part.Length = &length
	} else if !more {
		length := offset + uint64(end)
		part.Length = &length
	}
	return part.Document(), nil
}

// textual returns true when the type is text's, which is read as its text
func textual(contentType string) bool {
	mediatype, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return false
	}
	switch {
	case strings.HasPrefix(mediatype, "text/"):
		return true
	case mediatype == "application/json", mediatype == "application/xml", mediatype == "application/yaml",
		mediatype == "application/x-yaml", mediatype == "application/javascript", mediatype == "application/toml":
		return true
	case strings.HasSuffix(mediatype, "+json"), strings.HasSuffix(mediatype, "+xml"):
		return true
	}
	return false
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
