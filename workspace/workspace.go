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

	// List returns the workspace's resources, in a folder, a page at a time,
	// in an order which is the same each time, so its pages don't overlap
	List(ctx context.Context, req ListRequest) (*ListResponse, error)
}

///////////////////////////////////////////////////////////////////////////////
// TYPES

// ListRequest is which of a workspace's resources are listed: those in a
// folder, or under it, from a cursor, and at most a limit of them
type ListRequest struct {
	// Path is the folder whose resources are listed, such as "notes/2026",
	// or empty, for the workspace's top
	Path string `json:"path,omitempty"`

	// Recursive lists every resource under the folder, rather than only
	// those in it
	Recursive bool `json:"recursive,omitempty"`

	// Cursor is where the list continues from, which is a response's Next,
	// or empty, to start
	Cursor string `json:"cursor,omitempty"`

	// Limit is the most resources which are returned, or zero, for the
	// workspace's own page size
	Limit uint `json:"limit,omitempty"`
}

// ListResponse is a page of a workspace's resources, and the cursor of the
// next page
type ListResponse struct {
	// Resources are the page's resources
	Resources []llm.Resource `json:"-"`

	// Next is the cursor of the next page, or empty, when there isn't one
	Next string `json:"next,omitempty"`
}
