package schema

import (
	"time"

	// Packages
	types "github.com/mutablelogic/go-server/pkg/types"
)

////////////////////////////////////////////////////////////////////////////////
// TYPES

// Represents an LLM model
type Model struct {
	Name        string         `json:"name,omitzero"`
	Description string         `json:"description,omitzero"`
	Created     time.Time      `json:"created,omitzero"`
	OwnedBy     string         `json:"owned_by,omitzero"` // Model provider
	Aliases     []string       `json:"aliases,omitzero"`  // Model aliases
	Meta        map[string]any `json:"meta,omitzero"`     // Provider-specific metadata
}

////////////////////////////////////////////////////////////////////////////////
// STRINGIFY

func (m Model) String() string {
	return types.Stringify(m)
}
