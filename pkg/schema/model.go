package schema

import "time"

////////////////////////////////////////////////////////////////////////////////
// TYPES

// Represents an LLM model
type Model struct {
	Name        string
	Description string
	Created     time.Time `json:",omitzero"`
	OwnedBy     string
	Aliases     []string `json:",omitzero"`
}

////////////////////////////////////////////////////////////////////////////////
// STRINGIFY

func (m Model) String() string {
	return Stringify(m)
}
