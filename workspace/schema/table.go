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
	return []string{"Workspace", "Title", "Version", "Features", "Description"}
}

func (Workspace) Width(int) int {
	return 0
}

// Cell returns the workspace's name, title, version, features, and its
// description
func (w Workspace) Cell(col int) tui.CellText {
	switch col {
	case 0:
		return w.Name
	case 1:
		return w.Title
	case 2:
		return w.Version
	case 3:
		return strings.Join(w.Features(), ", ")
	case 4:
		return w.Description
	}
	return ""
}

// Features returns what the workspace does, besides listing its folders and
// resources: read, when its resources' content is read, by their URIs, and
// watch, when its changes are followed, and published, now
func (w Workspace) Features() []string {
	var result []string
	if w.Read {
		result = append(result, "read")
	}
	if w.Watch {
		result = append(result, "watch")
	}
	return result
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
