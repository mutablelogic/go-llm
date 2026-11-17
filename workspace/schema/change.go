package schema

import (
	// Packages
	eventschema "github.com/mutablelogic/go-server/broadcaster/schema"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Change is a folder, or resource, of a workspace which was created,
// updated, or deleted, or, without a URI, that its folders, and resources,
// changed, but not which, so they're listed again, as an MCP server's
// list_changed says
type Change struct {
	// Action is what happened: create, update, or delete
	Action eventschema.ChangeAction `json:"action"`

	// URI is the folder's, or resource's, or empty, when it isn't known
	URI string `json:"uri,omitempty"`
}
