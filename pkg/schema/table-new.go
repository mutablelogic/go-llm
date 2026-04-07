package schema

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	uuid "github.com/google/uuid"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// AGENT TABLE

func (AgentMeta) Header() []string {
	return []string{"NAME", "DESCRIPTION", "MODEL", "TOOLS"}
}

func (AgentMeta) Width(i int) int {
	switch i {
	case 0:
		return 24
	case 1:
		return 40
	case 2:
		return 24
	case 3:
		return 24
	}
	return 0
}

func (a AgentMeta) Cell(i int) string {
	switch i {
	case 0:
		return a.Name
	case 1:
		return toolDescription(ToolMeta{Title: a.Title, Description: a.Description})
	case 2:
		if a.Provider != nil && *a.Provider != "" && a.Model != nil && *a.Model != "" {
			return *a.Provider + "/" + *a.Model
		}
		if a.Model != nil && *a.Model != "" {
			return *a.Model
		}
		if a.Provider != nil {
			return *a.Provider
		}
		return ""
	case 3:
		return strings.Join(a.Tools, ", ")
	}
	return ""
}

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

///////////////////////////////////////////////////////////////////////////////
// SESSION TABLE

func (Session) Header() []string {
	return []string{"ID", "TITLE", "MODEL", "TAGS", "PARENT", "MODIFIED"}
}

func (Session) Width(i int) int {
	switch i {
	case 0:
		return 36
	case 1:
		return 24
	case 2:
		return 24
	case 3:
		return 24
	case 4:
		return 36
	case 5:
		return 19
	}
	return 0
}

func (s Session) Cell(i int) string {
	switch i {
	case 0:
		return s.ID.String()
	case 1:
		return types.Value(s.Title)
	case 2:
		if s.Provider != nil && *s.Provider != "" && s.Model != nil && *s.Model != "" {
			return *s.Provider + "/" + *s.Model
		}
		if s.Model != nil {
			return *s.Model
		}
		if s.Provider != nil {
			return *s.Provider
		}
	case 3:
		return strings.Join(s.Tags, ", ")
	case 4:
		if s.Parent != uuid.Nil {
			return s.Parent.String()
		}
	case 5:
		if s.ModifiedAt != nil {
			return s.ModifiedAt.Format("2006-01-02 15:04:05")
		}
		if !s.CreatedAt.IsZero() {
			return s.CreatedAt.Format("2006-01-02 15:04:05")
		}
	}
	return ""
}

///////////////////////////////////////////////////////////////////////////////
// MESSAGE TABLE

func (Message) Header() []string {
	return []string{"ROLE", "TEXT", "TOKENS", "RESULT"}
}

func (Message) Width(i int) int {
	switch i {
	case 0:
		return 10
	case 1:
		return 72
	case 2:
		return 8
	case 3:
		return 14
	}
	return 0
}

func (m Message) Cell(i int) string {
	switch i {
	case 0:
		return m.Role
	case 1:
		return truncateTableText(messageTableText(m), 280)
	case 2:
		if m.Tokens > 0 {
			return fmt.Sprintf("%d", m.Tokens)
		}
	case 3:
		if result := m.Result.String(); result != "" && result != "stop" && result != "unknown" {
			return result
		}
	}
	return ""
}

func messageTableText(m Message) string {
	parts := make([]string, 0, len(m.Content))
	for _, block := range m.Content {
		switch {
		case block.Text != nil:
			if text := compactTableText(*block.Text); text != "" {
				parts = append(parts, text)
			}
		case block.Thinking != nil:
			if text := compactTableText(*block.Thinking); text != "" {
				parts = append(parts, "[thinking] "+text)
			}
		case block.ToolCall != nil:
			parts = append(parts, "[tool call] "+block.ToolCall.Name)
		case block.ToolResult != nil:
			parts = append(parts, "[tool result] "+truncateTableText(compactTableText(string(block.ToolResult.Content)), 120))
		case block.Attachment != nil:
			attachment := "[attachment]"
			if block.Attachment.ContentType != "" {
				attachment += " " + block.Attachment.ContentType
			}
			if block.Attachment.URL != nil && block.Attachment.URL.String() != "" {
				attachment += " " + block.Attachment.URL.String()
			}
			parts = append(parts, attachment)
		}
	}
	return strings.Join(parts, " ")
}

func compactTableText(text string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(text)), " ")
}

func truncateTableText(text string, limit int) string {
	text = strings.TrimSpace(text)
	if limit <= 0 || text == "" {
		return text
	}
	runes := []rune(text)
	if len(runes) <= limit {
		return text
	}
	return strings.TrimSpace(string(runes[:limit-1])) + "..."
}
