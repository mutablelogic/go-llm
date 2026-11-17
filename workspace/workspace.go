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

// Workspace is a set of resources, such as a folder, or a vault of notes.
// Its name, title and description are its namespace's, or MCP server's,
// and it's added to a registry by its name.
type Workspace interface {
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

// Watcher is a workspace whose folders, and resources, change, which it
// says, which a workspace doesn't need to be
type Watcher interface {
	Workspace

	// WatchResources sends each folder, and resource, which is created, updated, or
	// deleted, to the channel, until the context is cancelled, when it
	// returns nil, or it fails. Each is waited for the channel to receive,
	// so none is missed, and the channel is read until the context is
	// cancelled. A change without a URI says the workspace changed, but not
	// where, so it's listed again, which it sends first, once it's watching,
	// so a change after that isn't missed.
	WatchResources(ctx context.Context, ch chan<- schema.Change) error
}

// Available is a workspace which is one only some of the time, such as an
// MCP server's namespace, which is while it's connected to a server which
// says it's a workspace. A registry has it only while it's available.
type Available interface {
	Workspace

	// Available returns true when it's a workspace, now
	Available() bool
}

// Subscriber is a watcher which says a resource is updated only once it's
// subscribed to, such as an MCP server's, which a server of it subscribes
// to, as its own clients do
type Subscriber interface {
	Watcher

	// Subscribe asks for the resource with the URI's updates
	Subscribe(ctx context.Context, uri string) error

	// Unsubscribe stops asking for the resource with the URI's updates
	Unsubscribe(ctx context.Context, uri string) error
}

// Describer is a workspace which says what it is, for people: its title,
// description, and version, such as an MCP connector's namespace, whose
// title, and version, are its server's, and whose description is its
// namespace's, which a workspace doesn't need to be
// Rooted is a workspace whose resources' URIs are under a root, such as
// file:///, which a registry leaves out of their URIs, such as
// resource://files/notes/a.md, of file:///notes/a.md
type Rooted interface {
	Workspace

	// Root returns the root of its resources' URIs, which ends with a
	// slash, or nothing, when it has none
	Root() string
}

type Describer interface {
	Workspace

	// Title returns the workspace's title, or nothing
	Title() string

	// Description returns what the workspace has, or nothing
	Description() string

	// Version returns the workspace's version, such as its server's, or
	// nothing
	Version() string
}
