package llm

import (
	"encoding/json"
	"time"
)

////////////////////////////////////////////////////////////////////////////////
// TYPES

// Represents an LLM model
type Model interface {
	Name() string
	Provider() Provider // Model provider
	Description() string
	Created() time.Time
	Aliases() []string                // Model aliases
	Meta() map[string]json.RawMessage // Provider-specific metadata
	InputTokenLimit() *uint           // Input token limit (optional)
	OutputTokenLimit() *uint          // Output token limit (optional)
}
