package schema

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Workspace is a workspace in a registry: its name, and whether its
// resources are read, and its changes are watched
type Workspace struct {
	Name        string `json:"name" help:"Name of the workspace, which is its namespace's"`
	Title       string `json:"title,omitempty" help:"Title of the workspace, such as its MCP server's"`
	Description string `json:"description,omitempty" help:"What the workspace has, as its namespace says"`
	Version     string `json:"version,omitempty" help:"Version of the workspace, such as its MCP server's"`
	Read        bool   `json:"read,omitempty" help:"Whether its resources' content is read, by their URIs, rather than only listed"`
	Watch       bool   `json:"watch,omitempty" help:"Whether its changes are followed, and published as events, now"`
}

// WorkspaceList is the workspaces, in order of their names, and how many
// there are
type WorkspaceList struct {
	Count uint        `json:"count" help:"How many workspaces there are"`
	Body  []Workspace `json:"body" help:"The workspaces"`
}
