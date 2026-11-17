package memory

import (
	"context"
	"fmt"
	"strconv"
	"strings"
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

// document is a resource of the memory's workspace, as Markdown
type document struct {
	uri, name string
	data      []byte
}

var _ workspace.Reader = (*Memory)(nil)
var _ workspace.Watcher = (*Memory)(nil)
var _ workspace.Describer = (*Memory)(nil)
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

	// The details, and the folder, at the top, and the memories in the
	// folder, or under the top
	var all []workspaceschema.ResourceMeta
	if path == "" {
		details := m.detailsDocument()
		all = append(all,
			workspaceschema.ResourceMeta{URI: DetailsURI, Name: details.name, Title: "Details", Description: "The details of the person, and the assistant", Type: markdownType, Size: int64(len(details.data))},
			workspaceschema.ResourceMeta{URI: MemoriesURI, Name: memoriesFolder, Title: "Memories", Type: workspaceschema.FolderType},
		)
	}
	if path == memoriesFolder || req.Recursive {
		memories, err := m.store.ListMemories(ctx, "")
		if err != nil {
			return nil, err
		}
		for _, memory := range memories {
			memory.Version = memory.Hash()
			// When it was changed, or remembered, when it hasn't been since
			doc := memoryDocument(memory)
			modified := memory.Modified
			if modified.IsZero() {
				modified = memory.Created
			}
			all = append(all, workspaceschema.ResourceMeta{URI: doc.uri, Name: doc.name, Title: memory.Key, Description: memory.Description, Type: markdownType, Size: int64(len(doc.data)), Modified: modified})
		}
	}

	// A page of them, from the cursor
	start := 0
	if req.Cursor != "" {
		n, err := strconv.Atoi(req.Cursor)
		if err != nil || n < 0 {
			return nil, llm.ErrBadParameter.Withf("cursor %q isn't valid", req.Cursor)
		}
		start = min(n, len(all))
	}
	limit := int(schema.MemoryListMax)
	if req.Limit > 0 {
		limit = min(int(req.Limit), limit)
	}
	end := min(start+limit, len(all))
	response := &workspaceschema.ListResponse{Body: all[start:end]}
	if end < len(all) {
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
// channel, as the memory publishes them to its broadcaster, until the
// context is cancelled, when it returns nil. It sends a change without a
// URI first, once it's watching. Returns ErrNotImplemented when the memory
// has no broadcaster.
func (m *Memory) WatchResources(ctx context.Context, ch chan<- workspaceschema.Change) error {
	if ch == nil {
		return llm.ErrBadParameter.With("missing channel")
	} else if m.broadcaster == nil {
		return llm.ErrNotImplemented.With("the memory has no broadcaster")
	}
	events := make(chan eventschema.Event, 16)
	done, err := m.broadcaster.Watch(ctx, events)
	if err != nil {
		return err
	}
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
		case event := <-events:
			if change, ok := changeOf(event); ok && !send(change) {
				return nil
			}
		case err := <-done:
			if ctx.Err() != nil {
				return nil
			}
			return err
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
	name := memory.Key + ".md"
	return &document{uri: MemoriesURI + name, name: name, data: []byte(b.String())}
}

// title returns the field's name, as a title, such as Timezone
func title(field schema.Field) string {
	name := string(field)
	if name == "" {
		return name
	}
	return strings.ToUpper(name[:1]) + name[1:]
}

// changeOf returns the memory's event as a change of its workspace: a
// memory which is created, updated, or deleted, or the details, which are
// updated, as one is
func changeOf(event eventschema.Event) (workspaceschema.Change, bool) {
	if event.Schema != Name {
		return workspaceschema.Change{}, false
	}
	switch event.Table {
	case MemoryTable:
		return workspaceschema.Change{Action: event.Action, URI: MemoriesURI + event.Row + ".md"}, true
	case DetailTable:
		return workspaceschema.Change{Action: eventschema.EventUpdate, URI: DetailsURI}, true
	}
	return workspaceschema.Change{}, false
}
