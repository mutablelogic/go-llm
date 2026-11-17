package schema

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Workspace is a workspace in a registry: its name, and whether its
// resources are read, and its changes are watched
type Workspace struct {
	Name  string `json:"name" help:"Name of the workspace, which is its namespace's"`
	Read  bool   `json:"read,omitempty" help:"Whether its resources are read"`
	Watch bool   `json:"watch,omitempty" help:"Whether its changes are watched"`
}

// WorkspaceList is the workspaces, in order of their names, and how many
// there are
type WorkspaceList struct {
	Count uint        `json:"count" help:"How many workspaces there are"`
	Body  []Workspace `json:"body" help:"The workspaces"`
}
