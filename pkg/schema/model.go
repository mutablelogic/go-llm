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
	Name             string         `json:"name,omitzero"`
	Description      string         `json:"description,omitzero"`
	Created          time.Time      `json:"created,omitzero"`
	OwnedBy          string         `json:"owned_by,omitzero"`           // Model provider
	Aliases          []string       `json:"aliases,omitzero"`            // Model aliases
	Meta             map[string]any `json:"meta,omitzero"`               // Provider-specific metadata
	InputTokenLimit  *uint          `json:"input_token_limit,omitzero"`  // Input token limit (optional)
	OutputTokenLimit *uint          `json:"output_token_limit,omitzero"` // Output token limit (optional)
	Cap              ModelCap       `json:"capabilities,omitzero"`       // Model capabilities (optional)
}

// Model Capabilities
type ModelCap uint

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	ModelCapEmbeddings ModelCap = 1 << iota
	ModelCapCompletion
	ModelCapThinking
	ModelCapTools
	ModelCapVision
	ModelCapTranscription
	ModelCapTranslation
)

////////////////////////////////////////////////////////////////////////////////
// STRINGIFY

func (m Model) String() string {
	return types.Stringify(m)
}
