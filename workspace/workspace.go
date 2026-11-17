// Package workspace is a registry of workspaces: sets of resources, such as
// a folder, or a vault of notes, which are listed, and later read, searched
// and watched, and served, such as over MCP.
package workspace

import (
	"context"
	"encoding/json"
	"time"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	metadata "github.com/mutablelogic/go-metadata"
)

///////////////////////////////////////////////////////////////////////////////
// INTERFACES

// Workspace is a set of resources, such as a folder, or a vault of notes
type Workspace interface {
	// Name returns the workspace's name, which is an identifier, and is
	// unique in a registry
	Name() string

	// Title returns the workspace's title, for people, or nothing
	Title() string

	// Description returns what the workspace has, or nothing
	Description() string

	// List returns the workspace's folders and resources, in a folder, or
	// under it, a page at a time, in an order which is the same each time,
	// so its pages don't overlap. A folder is a resource whose type is
	// FolderType, so a tree is made of what's listed.
	List(ctx context.Context, req ListRequest) (*ListResponse, error)
}

// Reader is a workspace whose resources, and artwork, are read, by their
// URIs, which a workspace doesn't need to be
type Reader interface {
	Workspace

	// Read returns the resource, or artwork, with the URI, as a resource, or
	// its artwork, has it, whose content is read as it's read. Returns
	// ErrNotFound when there's none, and ErrBadParameter for a folder,
	// which has no content.
	Read(ctx context.Context, uri string) (llm.Resource, error)
}

///////////////////////////////////////////////////////////////////////////////
// TYPES

// ListRequest is which of a workspace's folders and resources are listed:
// those in a folder, or under it, from a cursor, and at most a limit of them
type ListRequest struct {
	// Path is the folder whose folders and resources are listed, such as
	// "notes/2026", or empty, for the workspace's top
	Path string `json:"path,omitempty"`

	// Recursive lists every folder and resource under the folder, rather
	// than only those in it
	Recursive bool `json:"recursive,omitempty"`

	// Cursor is where the list continues from, which is a response's Next,
	// or empty, to start
	Cursor string `json:"cursor,omitempty"`

	// Limit is the most folders and resources which are returned, or zero, for the
	// workspace's own page size
	Limit uint `json:"limit,omitempty"`
}

// ListResponse is a page of a workspace's folders and resources, and the
// cursor of the next page
type ListResponse struct {
	// Body is the page's folders and resources
	Body []ResourceMeta `json:"body"`

	// Next is the cursor of the next page, or empty, when there isn't one
	Next string `json:"next,omitempty"`
}

// ResourceMeta is a folder, or a resource, of a workspace, as it's listed:
// what it is, rather than its content
type ResourceMeta struct {
	// URI is the resource's URI, such as file:///notes/2026/meeting.md
	URI string `json:"uri"`

	// Name is the resource's name, such as meeting.md
	Name string `json:"name"`

	// Title is the resource's title, for people, or nothing
	Title string `json:"title,omitempty"`

	// Description is what the resource is, or nothing
	Description string `json:"description,omitempty"`

	// Type is the MIME type of the resource's content, or FolderType, for a
	// folder, or nothing, when it isn't known
	Type string `json:"type,omitempty"`

	// Size is the length of the resource's content, in bytes, or zero, when
	// it isn't known
	Size int64 `json:"size,omitempty"`

	// Modified is when the resource was last changed, or zero, when it isn't
	// known
	Modified time.Time `json:"modified,omitzero"`

	// Meta is the resource's metadata, of which a key can have more than one
	// value
	Meta []Meta `json:"meta,omitempty"`

	// Artwork is the resource's images, such as its cover, or thumbnail
	Artwork []Artwork `json:"artwork,omitempty"`
}

// Meta is a value of a resource's metadata, by its namespace and key, such as
// exif:DateTimeOriginal, as text, to show, and as JSON
type Meta struct {
	// NS is the namespace of the key, such as exif
	NS string `json:"ns"`

	// Key is the key, in the namespace, such as DateTimeOriginal
	Key string `json:"key"`

	// Text is the value, as text, to show
	Text string `json:"text"`

	// Value is the value, as JSON, such as a number, a date, or a list
	Value json.RawMessage `json:"value,omitempty"`
}

// Artwork is an image of a resource, with what it is, such as its cover, or
// a thumbnail, which is read by its URI
type Artwork struct {
	// Key is what the image is, such as cover, or thumbnail, as go-metadata's
	// artwork keys are
	Key string `json:"key"`

	// Type is the image's MIME type, such as image/jpeg
	Type string `json:"type,omitempty"`

	// Width and Height are the image's size, in pixels, or zero, when it
	// isn't known
	Width  int `json:"width,omitempty"`
	Height int `json:"height,omitempty"`

	// URI is where the image is read from
	URI string `json:"uri"`
}

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

// FolderType is the type of a folder, which is listed as a resource, as it's
// the MIME type of a directory
const FolderType = "inode/directory"

///////////////////////////////////////////////////////////////////////////////
// PUBLIC FUNCTIONS

// IsFolder returns true when the resource is a folder, as its type says
func (r ResourceMeta) IsFolder() bool {
	return r.Type == FolderType
}

// MetaOf returns go-metadata's metadata as a resource's, apart from its
// artwork, whose images are the resource's Artwork, with its value as text,
// and as JSON, which is the text, as a JSON string, when its value can't be
// encoded as JSON
func MetaOf(md []metadata.Metadata) []Meta {
	result := make([]Meta, 0, len(md))
	for _, m := range md {
		if m == nil || m.NS() == metadata.ArtworkNamespace {
			continue
		}
		text := m.Value()
		value, err := json.Marshal(m.Any())
		if err != nil {
			value, _ = json.Marshal(text)
		}
		result = append(result, Meta{NS: m.NS(), Key: m.Key(), Text: text, Value: value})
	}
	return result
}
