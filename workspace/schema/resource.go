// Package schema has the types of workspaces: what's listed of their
// folders and resources, with their metadata and artwork
package schema

import (
	"encoding/json"
	"net/url"
	"strconv"
	"time"

	// Packages
	metadata "github.com/mutablelogic/go-metadata"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// ListRequest is which of a workspace's folders and resources are listed:
// those in a folder, or under it, from a cursor, and at most a limit of them
type ListRequest struct {
	// Path is the folder whose folders and resources are listed, such as
	// "notes/2026", or empty, for the workspace's top
	Path string `json:"path,omitempty" help:"The folder whose folders and resources are listed, or empty, for the top" optional:""`

	// Recursive lists every folder and resource under the folder, rather
	// than only those in it
	Recursive bool `json:"recursive,omitempty" help:"Whether every folder and resource under the folder is listed, rather than only those in it" optional:""`

	// Cursor is where the list continues from, which is a response's Next,
	// or empty, to start
	Cursor string `json:"cursor,omitempty" help:"Where the list continues from, which is a response's next, or empty, to start" optional:""`

	// Limit is the most folders and resources which are returned, or zero, for the
	// workspace's own page size
	Limit uint `json:"limit,omitempty" help:"Most folders and resources to return, or zero, for the workspace's own page size" optional:""`

	// Type is the type of the folders, or resources, which are listed, such
	// as FolderType, for its folders alone, or empty, for all of them
	Type string `json:"type,omitempty" help:"Only the folders, or resources, of the type, such as inode/directory, for the folders alone, or all of them" optional:""`
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

	// Path is the resource's path, under the workspace's top, such as
	// notes/2026, which a folder is listed by, or nothing, when it isn't
	// known
	Path string `json:"path,omitempty"`

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

// Query returns the request as the query of a URL, without its fields which
// aren't set, and with its folder, and cursor, as they are, as a folder's
// name can have spaces, and a cursor is the workspace's own
func (r ListRequest) Query() url.Values {
	q := url.Values{}
	if r.Path != "" {
		q.Set("path", r.Path)
	}
	if r.Recursive {
		q.Set("recursive", "true")
	}
	if r.Cursor != "" {
		q.Set("cursor", r.Cursor)
	}
	if r.Limit > 0 {
		q.Set("limit", strconv.FormatUint(uint64(r.Limit), 10))
	}
	return q
}

// ReadRequest is how a workspace's resource is read: by its URI, which is
// the workspace's, resource://<workspace>/<uri>, or its own, as it is, or
// as a Markdown document, a part at a time, and whether it's downloaded
type ReadRequest struct {
	URI      string `json:"uri" help:"URI of the resource, as it's listed, resource://<workspace>/<uri>, or its own"`
	Format   string `json:"format,omitempty" enum:"raw,markdown" help:"raw, for the resource as it is, which is the default, or markdown, for its text, as a Markdown document, a part at a time" optional:""`
	Offset   uint64 `json:"offset,omitempty" help:"With markdown, the byte of its Markdown the part starts at, which is a part's next" optional:""`
	Limit    uint64 `json:"limit,omitempty" help:"With markdown, the most bytes of its Markdown which are read" optional:""`
	Download bool   `json:"download,omitempty" help:"Whether the content is downloaded, as an attachment, which the browser saves, rather than shown in it" optional:""`
}

// Query returns the request as the query of a URL, without its fields which
// aren't set
func (r ReadRequest) Query() url.Values {
	q := url.Values{}
	if r.URI != "" {
		q.Set("uri", r.URI)
	}
	if r.Format != "" {
		q.Set("format", r.Format)
	}
	if r.Offset > 0 {
		q.Set("offset", strconv.FormatUint(r.Offset, 10))
	}
	if r.Limit > 0 {
		q.Set("limit", strconv.FormatUint(r.Limit, 10))
	}
	if r.Download {
		q.Set("download", "true")
	}
	return q
}
