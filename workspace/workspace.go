// Package workspace is a registry of workspaces: sets of resources, such as
// a folder, or a vault of notes, which are listed, and later read, searched
// and watched, and served, such as over MCP.
package workspace

import (
	"context"

	// Packages
	llm "github.com/mutablelogic/go-llm"
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
	// FolderType, as NewFolder's is, so a tree is made of what's listed.
	List(ctx context.Context, req ListRequest) (*ListResponse, error)
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

	// Limit is the most resources which are returned, or zero, for the
	// workspace's own page size
	Limit uint `json:"limit,omitempty"`
}

// ListResponse is a page of a workspace's folders and resources, and the
// cursor of the next page
type ListResponse struct {
	// Resources are the page's folders, whose type is FolderType, and
	// resources
	Resources []llm.Resource `json:"-"`

	// Next is the cursor of the next page, or empty, when there isn't one
	Next string `json:"next,omitempty"`
}

// folder is a folder of a workspace, as it's listed: a resource whose type
// is FolderType, which has no content of its own
type folder struct {
	uri, name, description string
}

var _ llm.Resource = (*folder)(nil)

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

// FolderType is the type of a folder, which is listed as a resource, as
// it's the MIME type of a directory
const FolderType = "inode/directory"

///////////////////////////////////////////////////////////////////////////////
// PUBLIC FUNCTIONS

// NewFolder returns a folder, as it's listed: a resource with the URI, such
// as file:///notes/2026/, the name, such as "2026", and the description, or
// none, whose type is FolderType, and which has no content of its own
func NewFolder(uri, name, description string) llm.Resource {
	return &folder{uri: uri, name: name, description: description}
}

// IsFolder returns true when the resource is a folder, as its type says
func IsFolder(r llm.Resource) bool {
	return r != nil && r.Type() == FolderType
}

///////////////////////////////////////////////////////////////////////////////
// FOLDER

func (f *folder) URI() string         { return f.uri }
func (f *folder) Name() string        { return f.name }
func (f *folder) Description() string { return f.description }
func (f *folder) Type() string        { return FolderType }

// Read returns ErrBadParameter, as a folder has no content: its folders and
// resources are listed
func (f *folder) Read(context.Context) (llm.ReadFunc, error) {
	return nil, llm.ErrBadParameter.Withf("%s is a folder", f.uri)
}
