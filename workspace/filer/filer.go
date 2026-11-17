// Package filer is a workspace of a go-filer backend, such as a directory,
// or a bucket, which lists its folders and files, reads them, and watches
// them, when the backend can, and an MCP server of it
package filer

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"net/url"
	"path"
	"strings"
	"sync"

	// Packages
	gofiler "github.com/mutablelogic/go-filer"
	filerschema "github.com/mutablelogic/go-filer/filer/schema"
	llm "github.com/mutablelogic/go-llm"
	eventschema "github.com/mutablelogic/go-llm/broadcaster/schema"
	workspace "github.com/mutablelogic/go-llm/workspace"
	schema "github.com/mutablelogic/go-llm/workspace/schema"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Workspace is the workspace of a go-filer backend: its folders, and files,
// whose URIs are file:// and their paths, such as file:///notes/a.md, and
// file:///notes/, for a folder, which are percent-encoded
type Workspace struct {
	name  string
	filer gofiler.Filer

	// The paths of the folders which were listed, or created, so a folder
	// which is deleted is said to be, by its URI, as the backend doesn't
	// say whether what's deleted was a folder
	folders sync.Map
}

// readyWatcher is a backend which says when it's watching, as go-filer's
// backends do
type readyWatcher interface {
	WatchObjectsReady(ctx context.Context, ch chan<- filerschema.Event, ready func()) error
}

// object is a file of the backend, which is read as it's read
type object struct {
	uri, name, mimeType string
	size                int64
	read                gofiler.ReadObjectFunc
}

var _ workspace.Reader = (*Workspace)(nil)
var _ workspace.Watcher = (*Workspace)(nil)
var _ llm.Sized = (*object)(nil)

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// DefaultLimit is how many folders, and files, are listed, when it isn't
	// said, and MaxLimit the most
	DefaultLimit = 100
	MaxLimit     = 1000

	// uriPrefix is the start of a folder's, or file's, URI, before its path
	uriPrefix = "file://"
)

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// New returns the workspace of the go-filer backend, whose volume has the
// name. Returns ErrBadParameter when there's no backend.
func New(name string, filer gofiler.Filer) (*Workspace, error) {
	if filer == nil {
		return nil, llm.ErrBadParameter.With("missing filer")
	}
	return &Workspace{name: name, filer: filer}, nil
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// List returns a page of the folders, and files, in the folder, or under
// it, in the order of their paths, from the cursor, which is the path of the
// last one of the page before
func (w *Workspace) List(ctx context.Context, req schema.ListRequest) (*schema.ListResponse, error) {
	limit := req.Limit
	if limit == 0 {
		limit = DefaultLimit
	}
	limit = min(limit, MaxLimit)
	folder := "/" + strings.Trim(req.Path, "/")
	iter, err := w.filer.ListObjects(ctx, filerschema.ListObjectsRequest{
		ObjectKey:  filerschema.ObjectKey{Name: w.name, Path: folder},
		Recursive:  req.Recursive,
		StartAfter: req.Cursor,
	})
	if err != nil {
		return nil, llmError(err)
	}
	defer iter.Close()

	// A page, and whether there's another
	result := &schema.ListResponse{Body: []schema.ResourceMeta{}}
	var last string
	for {
		obj, err := iter.Next(ctx)
		if errors.Is(err, io.EOF) {
			return result, nil
		} else if err != nil {
			return nil, llmError(err)
		} else if obj == nil || obj.Path == folder {
			// The folder isn't in itself
			continue
		} else if uint(len(result.Body)) == limit {
			result.Next = last
			return result, nil
		}
		// The cursor of a folder is its path, and a slash, as the backend
		// orders a folder, before what's in it
		last = obj.Path
		if obj.IsDir {
			w.folders.Store(strings.TrimSuffix(obj.Path, "/"), true)
			if !strings.HasSuffix(last, "/") {
				last += "/"
			}
		}
		result.Body = append(result.Body, resourceOf(obj))
	}
}

// Read returns the file with the URI, which is read as it's read. Returns
// ErrNotFound when there's none, and ErrBadParameter when it's a folder, or
// the URI isn't a file's.
func (w *Workspace) Read(ctx context.Context, uri string) (llm.Resource, error) {
	p, ok := pathOf(uri)
	if !ok {
		return nil, llm.ErrBadParameter.Withf("%q isn't a file's URI, file:///<path>", uri)
	}
	key := filerschema.ObjectKey{Name: w.name, Path: p}
	read, obj, err := w.filer.ReadObject(ctx, filerschema.GetObjectRequest{ObjectKey: key})
	if err != nil {
		// A folder can't be read, which is said, rather than why it can't
		if folder, err := w.filer.GetObject(ctx, filerschema.GetObjectRequest{ObjectKey: key}); err == nil && folder.IsDir {
			return nil, llm.ErrBadParameter.Withf("%s is a folder", uri)
		}
		return nil, llmError(err)
	} else if obj.IsDir {
		return nil, llm.ErrBadParameter.Withf("%s is a folder", uri)
	}
	resource := &object{uri: uriOf(obj.Path, false), name: path.Base(obj.Path), mimeType: obj.ContentType, size: -1, read: read}
	if obj.Size != nil {
		resource.size = *obj.Size
	}
	return resource, nil
}

// WatchResources sends the folders, and files, which are created, updated,
// or deleted, and, without a URI, that they changed, but not which, as the
// backend says a folder's changes may have been missed, to the channel,
// until the context is cancelled, when it returns nil. It sends a change
// without a URI first, once it's watching. Returns ErrNotImplemented when
// the backend doesn't watch its objects, or doesn't say when it's watching,
// so a change made once it's said to be isn't missed.
func (w *Workspace) WatchResources(ctx context.Context, ch chan<- schema.Change) error {
	if ch == nil {
		return llm.ErrBadParameter.With("missing channel")
	}
	send := func(change schema.Change) {
		select {
		case ch <- change:
		case <-ctx.Done():
		}
	}
	listAgain := schema.Change{Action: eventschema.EventUpdate}

	// The backend's events, which are sent as changes, as they're watched
	events := make(chan filerschema.Event, 16)
	done := make(chan error, 1)
	watcher, ok := w.filer.(readyWatcher)
	if !ok {
		return llm.ErrNotImplemented.With("the backend doesn't say when it's watching")
	}
	go func() { done <- watcher.WatchObjectsReady(ctx, events, func() { send(listAgain) }) }()
	for {
		select {
		case event := <-events:
			if change, ok := w.changeOf(ctx, event); ok {
				send(change)
			}
		case err := <-done:
			if err != nil && ctx.Err() == nil {
				return llmError(err)
			}
			return nil
		}
	}
}

///////////////////////////////////////////////////////////////////////////////
// OBJECT

func (o *object) URI() string         { return o.uri }
func (o *object) Name() string        { return o.name }
func (o *object) Description() string { return "" }
func (o *object) Type() string        { return o.mimeType }

// ContentLength returns the file's length, or -1, when it isn't known
func (o *object) ContentLength() int64 { return o.size }

// Read returns the function which reads the file's content, whose error,
// as the file is opened as it's read, is the llm error of its kind
func (o *object) Read(context.Context) (llm.ReadFunc, error) {
	return func(ctx context.Context, fn func(io.Reader) error) error {
		return llmError(o.read(ctx, fn))
	}, nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// changeOf returns the backend's event as a change: an object which is
// created, updated, or deleted, by its URI, which is a folder's, when it's
// one, or, as a folder's changes may have been missed, or what changed is
// gone before it's known whether it's a folder, that they changed, but not
// which
func (w *Workspace) changeOf(ctx context.Context, event filerschema.Event) (schema.Change, bool) {
	p := strings.TrimSuffix(event.Path, "/")
	switch event.EventType {
	case filerschema.EventTypeCreated, filerschema.EventTypeUpdated:
		obj, err := w.filer.GetObject(ctx, filerschema.GetObjectRequest{ObjectKey: event.ObjectKey})
		if err != nil {
			return schema.Change{Action: eventschema.EventUpdate}, true
		} else if obj.IsDir {
			w.folders.Store(p, true)
		}
		action := eventschema.EventCreate
		if event.EventType == filerschema.EventTypeUpdated {
			action = eventschema.EventUpdate
		}
		return schema.Change{Action: action, URI: uriOf(p, obj.IsDir)}, true
	case filerschema.EventTypeDeleted:
		_, folder := w.folders.LoadAndDelete(p)
		return schema.Change{Action: eventschema.EventDelete, URI: uriOf(p, folder)}, true
	case filerschema.EventTypeRescan:
		return schema.Change{Action: eventschema.EventUpdate}, true
	}
	return schema.Change{}, false
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// uriOf returns the URI of the path: file://, and the path, percent-encoded,
// which, for a folder, ends with a slash
func uriOf(p string, folder bool) string {
	if folder && !strings.HasSuffix(p, "/") {
		p += "/"
	}
	return (&url.URL{Scheme: "file", Path: p}).String()
}

// pathOf returns the path of the URI of a file, or a folder, decoded, and
// true, or false, when it isn't one. A URI which isn't percent-encoded, such
// as one which is typed, with a # or a %, is its path as it is.
func pathOf(uri string) (string, bool) {
	p, ok := strings.CutPrefix(uri, uriPrefix)
	if !ok || !strings.HasPrefix(p, "/") {
		return "", false
	}
	if u, err := url.Parse(uri); err == nil && u.Host == "" && u.RawQuery == "" && u.Fragment == "" && !u.ForceQuery {
		p = u.Path
	}
	if p != "/" {
		p = strings.TrimSuffix(p, "/")
	}
	return p, true
}

// resourceOf returns the object as a workspace's resource: its URI, name,
// type, which is a folder's, for one, size, when it was modified, and its
// metadata
func resourceOf(obj *filerschema.Object) schema.ResourceMeta {
	r := schema.ResourceMeta{
		URI:      uriOf(obj.Path, obj.IsDir),
		Name:     path.Base(obj.Path),
		Type:     obj.ContentType,
		Modified: obj.ModTime,
	}
	if obj.IsDir {
		r.Type = schema.FolderType
	} else if obj.Size != nil {
		r.Size = *obj.Size
	}
	for _, m := range obj.Meta {
		r.Meta = append(r.Meta, metaOf(m))
	}
	return r
}

// metaOf returns go-filer's metadata as a workspace's: its value as JSON,
// and as text, which is a string's own, or its JSON
func metaOf(m filerschema.Meta) schema.Meta {
	result := schema.Meta{NS: m.Namespace, Key: m.Key, Value: m.Value}
	var text string
	if err := json.Unmarshal(m.Value, &text); err == nil {
		result.Text = text
	} else {
		result.Text = string(m.Value)
	}
	return result
}

// llmError returns go-filer's error as the llm error of its kind
func llmError(err error) error {
	switch {
	case errors.Is(err, gofiler.ErrNotFound), errors.Is(err, fs.ErrNotExist):
		return llm.ErrNotFound.With(err.Error())
	case errors.Is(err, gofiler.ErrBadParameter):
		return llm.ErrBadParameter.With(err.Error())
	case errors.Is(err, gofiler.ErrNotImplemented):
		return llm.ErrNotImplemented.With(err.Error())
	case errors.Is(err, gofiler.ErrForbidden), errors.Is(err, fs.ErrPermission):
		return llm.ErrForbidden.With(err.Error())
	}
	return err
}
