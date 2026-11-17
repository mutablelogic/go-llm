package memory

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	eventschema "github.com/mutablelogic/go-llm/broadcaster/schema"
	schema "github.com/mutablelogic/go-llm/memory/schema"
	workspace "github.com/mutablelogic/go-llm/workspace"
	workspaceschema "github.com/mutablelogic/go-llm/workspace/schema"
	yaml "go.yaml.in/yaml/v3"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// watch is a watch of the memory's workspace: the changes which haven't
// been sent, by URI, in the order they happened, and a signal that there
// are more
type watch struct {
	mu      sync.Mutex
	pending []workspaceschema.Change
	signal  chan struct{}
}

// document is a resource of the memory's workspace, as Markdown
type document struct {
	uri, name string
	data      []byte
}

var _ workspace.Reader = (*Memory)(nil)
var _ workspace.Watcher = (*Memory)(nil)
var _ workspace.Describer = (*Memory)(nil)
var _ workspace.Rooted = (*Memory)(nil)
var _ llm.Sized = (*document)(nil)

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// DetailsURI is the URI of the details, in the memory's workspace
	DetailsURI = "memory:///details.md"

	// MemoriesURI is the URI of the folder of the memories, in the memory's
	// workspace, which each is in, by its key, such as
	// memory:///memories/kitchen_lights.md
	MemoriesURI = "memory:///memories/"

	// The folder of the memories, and the type of the documents
	memoriesFolder = "memories"
	markdownType   = "text/markdown"
)

///////////////////////////////////////////////////////////////////////////////
// WORKSPACE

// Title returns the title of the memory's workspace
func (*Memory) Title() string { return "Memory" }

// Root returns the root of the URIs of the memory's workspace, memory:///
func (*Memory) Root() string { return "memory:///" }

// Version returns nothing, as the memory has no version of its own
func (*Memory) Version() string { return "" }

// List returns the memory's workspace: its details, and the folder of its
// memories, the most recently changed first, a page at a time, whose cursor
// is how many were listed before it
func (m *Memory) List(ctx context.Context, req workspaceschema.ListRequest) (*workspaceschema.ListResponse, error) {
	path := strings.Trim(req.Path, "/")
	if path != "" && path != memoriesFolder {
		return nil, llm.ErrNotFound.Withf("no folder %q", req.Path)
	}

	// The details, and the folder, at the top, and then the memories, in the
	// folder, or under the top
	top := 0
	if path == "" {
		top = 2
	}
	var memories []schema.Memory
	if path == memoriesFolder || req.Recursive {
		var err error
		if memories, err = m.store.ListMemories(ctx, ""); err != nil {
			return nil, err
		}
	}
	total := top + len(memories)

	// A page of them, from the cursor, of at most the limit, which is
	// clamped before it's an int, so a large one isn't negative
	start := 0
	if req.Cursor != "" {
		n, err := strconv.Atoi(req.Cursor)
		if err != nil || n < 0 {
			return nil, llm.ErrBadParameter.Withf("cursor %q isn't valid", req.Cursor)
		}
		start = min(n, total)
	}
	limit := uint(schema.MemoryListMax)
	if req.Limit > 0 && req.Limit < limit {
		limit = req.Limit
	}
	end := min(start+int(limit), total)

	// Only the page's memories are rendered, for their sizes
	response := &workspaceschema.ListResponse{Body: make([]workspaceschema.ResourceMeta, 0, end-start)}
	for i := start; i < end; i++ {
		switch {
		case i == 0 && top > 0:
			details := m.detailsDocument()
			response.Body = append(response.Body, workspaceschema.ResourceMeta{URI: DetailsURI, Name: details.name, Title: "Details", Description: "The details of the person, and the assistant", Type: markdownType, Size: int64(len(details.data))})
		case i == 1 && top > 1:
			response.Body = append(response.Body, workspaceschema.ResourceMeta{URI: MemoriesURI, Name: memoriesFolder, Title: "Memories", Type: workspaceschema.FolderType})
		default:
			response.Body = append(response.Body, memoryMeta(memories[i-top]))
		}
	}
	if end < total {
		response.Next = strconv.Itoa(end)
	}
	return response, nil
}

// Read returns the details, or a memory, by its URI, as a Markdown document.
// Returns ErrNotFound when there's none, and ErrBadParameter for the folder.
func (m *Memory) Read(ctx context.Context, uri string) (llm.Resource, error) {
	switch {
	case uri == DetailsURI:
		return m.detailsDocument(), nil
	case uri == MemoriesURI || uri == strings.TrimSuffix(MemoriesURI, "/"):
		return nil, llm.ErrBadParameter.Withf("%s is a folder", uri)
	}
	key, ok := strings.CutPrefix(uri, MemoriesURI)
	if !ok || !strings.HasSuffix(key, ".md") {
		return nil, llm.ErrNotFound.Withf("no resource %q", uri)
	}
	memory, err := m.GetMemory(ctx, strings.TrimSuffix(key, ".md"))
	if err != nil {
		return nil, err
	}
	return memoryDocument(memory), nil
}

// WatchResources sends the details, and memories, which change, to the
// channel, until the context is cancelled, when it returns nil. It sends a
// change without a URI first, once it's watching. Changes which happen
// while one is sent are kept, by URI, so none is missed, however slowly
// they're received.
func (m *Memory) WatchResources(ctx context.Context, ch chan<- workspaceschema.Change) error {
	if ch == nil {
		return llm.ErrBadParameter.With("missing channel")
	}
	w := &watch{signal: make(chan struct{}, 1)}
	m.watchMu.Lock()
	if m.watches == nil {
		m.watches = make(map[*watch]struct{})
	}
	m.watches[w] = struct{}{}
	m.watchMu.Unlock()
	defer func() {
		m.watchMu.Lock()
		delete(m.watches, w)
		m.watchMu.Unlock()
	}()

	send := func(change workspaceschema.Change) bool {
		select {
		case ch <- change:
			return true
		case <-ctx.Done():
			return false
		}
	}
	if !send(workspaceschema.Change{Action: eventschema.EventUpdate}) {
		return nil
	}
	for {
		select {
		case <-w.signal:
			for _, change := range w.take() {
				if !send(change) {
					return nil
				}
			}
		case <-ctx.Done():
			return nil
		}
	}
}

///////////////////////////////////////////////////////////////////////////////
// DOCUMENT

func (d *document) URI() string          { return d.uri }
func (d *document) Name() string         { return d.name }
func (d *document) Description() string  { return "" }
func (d *document) Type() string         { return markdownType }
func (d *document) ContentLength() int64 { return int64(len(d.data)) }
func (d *document) Read(context.Context) (llm.ReadFunc, error) {
	return llm.ReadBytes(d.data), nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// notify tells each watch of the workspace of the change, without waiting
// for it
func (m *Memory) notify(change workspaceschema.Change) {
	m.watchMu.Lock()
	defer m.watchMu.Unlock()
	for w := range m.watches {
		w.add(change)
	}
}

// add keeps the change, as the latest of its URI's, unless it was created,
// and is updated, which is that it was created, and signals there are
// changes
func (w *watch) add(change workspaceschema.Change) {
	w.mu.Lock()
	i := slices.IndexFunc(w.pending, func(c workspaceschema.Change) bool { return c.URI == change.URI })
	switch {
	case i < 0:
		w.pending = append(w.pending, change)
	case w.pending[i].Action == eventschema.EventCreate && change.Action == eventschema.EventUpdate:
	default:
		w.pending[i] = change
	}
	w.mu.Unlock()
	select {
	case w.signal <- struct{}{}:
	default:
	}
}

// take returns the changes which haven't been sent, which are then
// forgotten
func (w *watch) take() []workspaceschema.Change {
	w.mu.Lock()
	defer w.mu.Unlock()
	pending := w.pending
	w.pending = nil
	return pending
}

// detailsDocument returns the details as a Markdown document: each which is
// known, or which the person declined to say
func (m *Memory) detailsDocument() *document {
	details := m.Details()
	var b strings.Builder
	b.WriteString("# Details\n\n")
	n := 0
	for _, row := range details.Rows() {
		switch {
		case row.Detail == nil:
			continue
		case row.Detail.Declined:
			fmt.Fprintf(&b, "- **%s:** declined to say\n", title(row.Field))
		default:
			fmt.Fprintf(&b, "- **%s:** %s\n", title(row.Field), row.Detail.Value)
		}
		n++
	}
	if n == 0 {
		b.WriteString("None are known.\n")
	}
	return &document{uri: DetailsURI, name: "details.md", data: []byte(b.String())}
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// memoryDocument returns the memory as a Markdown document, whose YAML front
// matter has its key, description, version, and when it was remembered, and
// last changed, and whose body is its body
func memoryDocument(memory schema.Memory) *document {
	front := struct {
		Key         string    `yaml:"key"`
		Description string    `yaml:"description,omitempty"`
		Version     string    `yaml:"version,omitempty"`
		Created     time.Time `yaml:"created,omitempty"`
		Modified    time.Time `yaml:"modified,omitempty"`
	}{memory.Key, memory.Description, memory.Version, memory.Created, memory.Modified}
	data, _ := yaml.Marshal(front)
	var b strings.Builder
	b.WriteString("---\n")
	b.Write(data)
	b.WriteString("---\n")
	if body := strings.TrimSpace(memory.Body); body != "" {
		b.WriteString("\n")
		b.WriteString(body)
		b.WriteString("\n")
	}
	return &document{uri: memoryURI(memory.Key), name: memory.Key + ".md", data: []byte(b.String())}
}

// title returns the field's name, as a title, such as Timezone
func title(field schema.Field) string {
	name := string(field)
	if name == "" {
		return name
	}
	return strings.ToUpper(name[:1]) + name[1:]
}

// memoryURI returns the URI of the memory with the key, in the memory's
// workspace
func memoryURI(key string) string {
	return MemoriesURI + key + ".md"
}

// memoryMeta returns the memory, as its workspace lists it: its document's
// URI, name, and size, its key, its description, and when it was changed,
// or remembered, when it hasn't been since
func memoryMeta(memory schema.Memory) workspaceschema.ResourceMeta {
	memory.Version = memory.Hash()
	doc := memoryDocument(memory)
	modified := memory.Modified
	if modified.IsZero() {
		modified = memory.Created
	}
	return workspaceschema.ResourceMeta{URI: doc.uri, Name: doc.name, Title: memory.Key, Description: memory.Description, Type: markdownType, Size: int64(len(doc.data)), Modified: modified}
}
