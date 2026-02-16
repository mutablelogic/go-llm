package schema

import (
	"fmt"

	uitable "github.com/mutablelogic/go-llm/pkg/ui/table"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// AgentTable implements table.TableData for a list of agents.
type AgentTable []*Agent

// SessionTable implements table.TableData for a list of sessions.
type SessionTable struct {
	Sessions       []*Session
	CurrentSession string
}

// ToolTable implements table.TableData for a list of tools.
type ToolTable []ToolMeta

// ModelTable implements table.TableData for a list of models.
type ModelTable struct {
	Models       []Model
	CurrentModel string
}

// ProviderTable implements table.TableData for a list of provider names.
type ProviderTable []string

///////////////////////////////////////////////////////////////////////////////
// AGENT TABLE (LIST)

func (t AgentTable) Header() []string {
	return []string{"AGENT", "ID", "TITLE", "DESCRIPTION"}
}

func (t AgentTable) Len() int {
	return len(t)
}

func (t AgentTable) Row(i int) []any {
	a := t[i]
	name := fmt.Sprintf("%s@%d", a.Name, a.Version)
	return []any{name, a.ID, a.Title, a.Description}
}

///////////////////////////////////////////////////////////////////////////////
// SESSION TABLE (LIST)

func (t SessionTable) Header() []string {
	return []string{"SESSION", "ID", "MODEL", "MESSAGES", "MODIFIED"}
}

func (t SessionTable) Len() int {
	return len(t.Sessions)
}

func (t SessionTable) Row(i int) []any {
	s := t.Sessions[i]
	messages := "-"
	if n := len(s.Messages); n > 0 {
		messages = fmt.Sprintf("%d (%d tokens)", n, s.Tokens())
	}
	row := []any{s.Name, s.ID, s.Model, messages, s.Modified}
	if t.CurrentSession != "" && s.ID == t.CurrentSession {
		for j, v := range row {
			row[j] = uitable.Bold{Value: v}
		}
	}
	return row
}

///////////////////////////////////////////////////////////////////////////////
// TOOL TABLE (LIST)

func (t ToolTable) Header() []string {
	return []string{"NAME", "DESCRIPTION"}
}

func (t ToolTable) Len() int {
	return len(t)
}

func (t ToolTable) Row(i int) []any {
	return []any{t[i].Name, t[i].Description}
}

///////////////////////////////////////////////////////////////////////////////
// MODEL TABLE (LIST)

func (t ModelTable) Header() []string {
	return []string{"NAME", "PROVIDER", "DESCRIPTION"}
}

func (t ModelTable) Len() int {
	return len(t.Models)
}

func (t ModelTable) Row(i int) []any {
	row := []any{t.Models[i].Name, t.Models[i].OwnedBy, t.Models[i].Description}
	if t.CurrentModel != "" && t.Models[i].Name == t.CurrentModel {
		for j, v := range row {
			row[j] = uitable.Bold{Value: v}
		}
	}
	return row
}

///////////////////////////////////////////////////////////////////////////////
// PROVIDER TABLE (LIST)

func (t ProviderTable) Header() []string {
	return []string{"PROVIDER"}
}

func (t ProviderTable) Len() int {
	return len(t)
}

func (t ProviderTable) Row(i int) []any {
	return []any{t[i]}
}
