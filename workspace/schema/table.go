package schema

import (
	"strings"

	// Packages
	tui "github.com/mutablelogic/go-server/pkg/tui"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// STRINGIFY

func (w Workspace) String() string     { return types.Stringify(w) }
func (w WorkspaceList) String() string { return types.Stringify(w) }
func (r ResourceMeta) String() string  { return types.Stringify(r) }
func (r ListResponse) String() string  { return types.Stringify(r) }
func (r ListRequest) String() string   { return types.Stringify(r) }

///////////////////////////////////////////////////////////////////////////////
// TABLE OUTPUT

func (Workspace) Header() []string {
	return []string{"Workspace", "Title", "Version", "Read", "Watch", "Description"}
}

func (Workspace) Width(int) int {
	return 0
}

// Cell returns the workspace's name, title, version, whether its resources
// are read, and its changes are watched, and its description
func (w Workspace) Cell(col int) tui.CellText {
	switch col {
	case 0:
		return w.Name
	case 1:
		return w.Title
	case 2:
		return w.Version
	case 3:
		return yes(w.Read)
	case 4:
		return yes(w.Watch)
	case 5:
		return w.Description
	}
	return ""
}

func (ResourceMeta) Header() []string {
	return []string{"Name", "Type", "Size", "Modified", "URI"}
}

func (ResourceMeta) Width(int) int {
	return 0
}

// Cell returns the resource's name, or title, which a folder ends with a
// slash, its type, size, when it was modified, and its URI
func (r ResourceMeta) Cell(col int) tui.CellText {
	switch col {
	case 0:
		name := r.Name
		if r.Title != "" {
			name = r.Title
		}
		if r.IsFolder() && !strings.HasSuffix(name, "/") {
			name += "/"
		}
		return name
	case 1:
		if r.IsFolder() {
			return ""
		}
		return r.Type
	case 2:
		if r.Size <= 0 {
			return ""
		}
		return r.Size
	case 3:
		if r.Modified.IsZero() {
			return ""
		}
		return r.Modified.Local().Format("2006-01-02 15:04")
	case 4:
		return r.URI
	}
	return ""
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// yes returns "yes", when it's true, or nothing
func yes(v bool) string {
	if v {
		return "yes"
	}
	return ""
}
