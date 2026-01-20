package schema

import "time"

////////////////////////////////////////////////////////////////////////////////
// TYPES

// Represents an LLM model
type Model struct {
	Name        string
	Description string
	Created     time.Time
	OwnedBy     string
	Aliases     []string
}

////////////////////////////////////////////////////////////////////////////////
// STRINGIFY

func (m Model) String() string {
	return stringify(m)
}
