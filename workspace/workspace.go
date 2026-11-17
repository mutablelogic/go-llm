// Package workspace is a registry of workspaces: sets of resources, such as
// a folder, or a vault of notes, which are listed, and later read, searched
// and watched, and served, such as over MCP.
package workspace

import (
	"context"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/workspace/schema"
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
	// schema.FolderType, so a tree is made of what's listed.
	List(ctx context.Context, req schema.ListRequest) (*schema.ListResponse, error)
}

// Reader is a workspace whose resources, and artwork, are read, by their
// URIs, which a workspace doesn't need to be
type Reader interface {
	Workspace

	// Read returns the resource, or artwork, with the URI which a listed
	// resource, or its artwork, has, whose content is read as it's read.
	// Returns ErrNotFound when there's none, and ErrBadParameter for a
	// folder, which has no content.
	Read(ctx context.Context, uri string) (llm.Resource, error)
}
