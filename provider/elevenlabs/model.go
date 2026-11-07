package elevenlabs

import (
	"context"
	"encoding/json"
	"time"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Model is one of ElevenLabs' Scribe models, which transcribe speech
type Model struct {
	provider    *Client
	name        string
	description string
}

var _ llm.Model = (*Model)(nil)

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

// models are the Scribe models, which ElevenLabs doesn't list, as its list
// of models is of those which speak, in order
// Reference: https://elevenlabs.io/docs/models
var models = []struct {
	name, description string
}{
	{"scribe_v2", "Scribe v2, which transcribes speech"},
	{"scribe_v2_medical", "Scribe v2, for medical speech"},
	{"scribe_v1", "Scribe v1, which transcribes speech"},
	{"scribe_v1_experimental", "Scribe v1, with experimental features"},
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Name returns the model's id, such as "scribe_v2"
func (m *Model) Name() string {
	return m.name
}

// Provider returns the client the model came from
func (m *Model) Provider() llm.Provider {
	return m.provider
}

// Description returns what the model is
func (m *Model) Description() string {
	return m.description
}

// Created isn't known
func (m *Model) Created() time.Time {
	return time.Time{}
}

// Aliases returns no other names
func (m *Model) Aliases() []string {
	return nil
}

// Meta returns no metadata
func (m *Model) Meta() map[string]json.RawMessage {
	return nil
}

// InputTokenLimit isn't known, as a model transcribes audio
func (m *Model) InputTokenLimit() *uint {
	return nil
}

// OutputTokenLimit isn't known, as a model transcribes audio
func (m *Model) OutputTokenLimit() *uint {
	return nil
}

// Capabilities returns what the model can do, which is transcribe speech,
// but not translate it, nor anything else
func (m *Model) Capabilities() llm.ModelCap {
	no := types.Ptr(false)
	return llm.ModelCap{
		Chat:          no,
		Tools:         no,
		Thinking:      no,
		ImageInput:    no,
		OCR:           no,
		Transcription: types.Ptr(true),
		Translation:   no,
		Embedding:     no,
	}
}

///////////////////////////////////////////////////////////////////////////////
// PROVIDER METHODS

// ListModels returns the Scribe models, which transcribe speech
func (c *Client) ListModels(context.Context) ([]llm.Model, error) {
	result := make([]llm.Model, 0, len(models))
	for _, model := range models {
		result = append(result, &Model{provider: c, name: model.name, description: model.description})
	}
	return result, nil
}

// GetModel returns the Scribe model with the name, or ErrNotFound
func (c *Client) GetModel(_ context.Context, name string) (llm.Model, error) {
	for _, model := range models {
		if model.name == name {
			return &Model{provider: c, name: model.name, description: model.description}, nil
		}
	}
	return nil, llm.ErrNotFound.Withf("model %q", name)
}
