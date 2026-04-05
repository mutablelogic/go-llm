package schema

import (
	"bytes"
	"encoding/json"
	"strings"

	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// PROVIDER TABLE

func (t Provider) Header() []string {
	return []string{"NAME", "PROVIDER", "URL", "ENABLED", "INCLUDE", "EXCLUDE", "CREATED AT", "MODIFIED AT"}
}

func (t Provider) Width(i int) int {
	switch i {
	case 0:
		return 20
	case 1:
		return 12
	case 2:
		return 32
	case 3:
		return 8
	case 4, 5:
		return 24
	case 6, 7:
		return 19
	}
	return 0
}

func (t Provider) Cell(i int) string {
	switch i {
	case 0:
		return t.Name
	case 1:
		return t.Provider
	case 2:
		if t.URL != nil {
			return *t.URL
		}
	case 3:
		if t.Enabled != nil && *t.Enabled {
			return "true"
		}
		return "false"
	case 4:
		return strings.Join(t.Include, ", ")
	case 5:
		return strings.Join(t.Exclude, ", ")
	case 6:
		if !t.CreatedAt.IsZero() {
			return t.CreatedAt.Format("2006-01-02 15:04:05")
		}
	case 7:
		if t.ModifiedAt != nil {
			return t.ModifiedAt.Format("2006-01-02 15:04:05")
		}
	}
	return ""
}

///////////////////////////////////////////////////////////////////////////////
// MODELS TABLE

func (Model) Header() []string {
	return []string{"NAME", "PROVIDER", "DESCRIPTION", "CAPABILITIES", "ALIASES"}
}

func (Model) Width(i int) int {
	switch i {
	case 0:
		return 24
	case 1:
		return 12
	case 2:
		return 40
	case 3:
		return 19
	case 4:
		return 24
	}
	return 0
}

func (m Model) Cell(i int) string {
	switch i {
	case 0:
		return m.Name
	case 1:
		return m.OwnedBy
	case 2:
		return m.Description
	case 3:
		if m.Cap != 0 {
			return m.Cap.String()
		}
	case 4:
		return strings.Join(m.Aliases, ", ")
	}
	return ""
}

///////////////////////////////////////////////////////////////////////////////
// TOOL TABLE

func (ToolMeta) Header() []string {
	return []string{"NAME", "DESCRIPTION", "INPUT", "OUTPUT", "HINTS"}
}

func (ToolMeta) Width(i int) int {
	switch i {
	case 0:
		return 24
	case 1:
		return 40
	case 2:
		return 36
	case 3:
		return 36
	case 4:
		return 24
	}
	return 0
}

func (t ToolMeta) Cell(i int) string {
	switch i {
	case 0:
		return t.Name
	case 1:
		return toolDescription(t)
	case 2:
		return compactSchema(t.Input)
	case 3:
		return compactSchema(t.Output)
	case 4:
		return strings.Join(t.Hints, ", ")
	}
	return ""
}

func toolDescription(t ToolMeta) string {
	title := strings.TrimSpace(t.Title)
	description := strings.TrimSpace(strings.ReplaceAll(t.Description, "\n", " "))
	switch {
	case title != "" && description != "":
		return title + " - " + description
	case title != "":
		return title
	default:
		return description
	}
}

func compactSchema(schema JSONSchema) string {
	if len(schema) == 0 {
		return ""
	}

	var buffer bytes.Buffer
	if err := json.Compact(&buffer, schema); err != nil {
		return strings.TrimSpace(strings.ReplaceAll(string(schema), "\n", " "))
	}

	return buffer.String()
}

///////////////////////////////////////////////////////////////////////////////
// CONNECTOR TABLE

func (Connector) Header() []string {
	return []string{"URL", "NAMESPACE", "TITLE", "ENABLED", "GROUPS", "CREATED AT", "MODIFIED AT"}
}

func (Connector) Width(i int) int {
	switch i {
	case 0:
		return 40
	case 1:
		return 16
	case 2:
		return 40
	case 3:
		return 8
	case 4:
		return 24
	case 5, 6:
		return 19
	}
	return 0
}

func (c Connector) Cell(i int) string {
	switch i {
	case 0:
		return c.URL
	case 1:
		if c.Namespace != nil {
			return *c.Namespace
		}
	case 2:
		var parts []string
		if s := types.Value(c.Name); s != "" {
			parts = append(parts, s)
		}
		if s := types.Value(c.Title); s != "" {
			parts = append(parts, s)
		}
		if s := types.Value(c.Description); s != "" {
			parts = append(parts, strings.ReplaceAll(s, "\n", " "))
		}
		if len(parts) > 0 {
			s := strings.Join(parts, " - ")
			if len(s) > 40 {
				return s[:37] + "..."
			}
			return s
		}
	case 3:
		if c.Enabled != nil && *c.Enabled {
			return "true"
		}
		return "false"
	case 4:
		return strings.Join(c.Groups, ", ")
	case 5:
		if !c.CreatedAt.IsZero() {
			return c.CreatedAt.Format("2006-01-02 15:04:05")
		}
	case 6:
		if c.ModifiedAt != nil {
			return c.ModifiedAt.Format("2006-01-02 15:04:05")
		}
	}
	return ""
}
