package elevenlabs

import (
	"context"
	"encoding/json"
	"maps"
	"time"

	// Packages
	client "github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Model is one of ElevenLabs' Scribe models, which transcribe speech, or one
// of the models it lists, which speak text
type Model struct {
	provider    *Client
	name        string
	description string
	speech      *apiModel
}

// apiModel is a model which speaks, as ElevenLabs lists it, with all its
// fields
type apiModel struct {
	ID          string `json:"model_id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Speaks      bool   `json:"can_do_text_to_speech"`
	MaxText     uint   `json:"maximum_text_length_per_request,omitempty"`
	Languages   []struct {
		ID   string `json:"language_id"`
		Name string `json:"name"`
	} `json:"languages,omitempty"`

	// All the fields of the model, including those above
	Fields map[string]json.RawMessage `json:"-"`
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

// Name returns the model's id, such as "scribe_v2", or "eleven_v3"
func (m *Model) Name() string {
	return m.name
}

// Provider returns the client the model came from
func (m *Model) Provider() llm.Provider {
	return m.provider
}

// Description returns what the model is, which, for a model which speaks,
// is ElevenLabs' description, or its name, such as "Eleven v3"
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

// Meta returns what ElevenLabs says of a model which speaks, such as its
// name, its languages, and the most characters it speaks at once, apart from
// its id, or nothing, for a Scribe model
func (m *Model) Meta() map[string]json.RawMessage {
	if m.speech == nil {
		return nil
	}
	meta := maps.Clone(m.speech.Fields)
	delete(meta, "model_id")
	if len(meta) == 0 {
		return nil
	}
	return meta
}

// InputTokenLimit isn't known, as a model transcribes audio, or speaks text
// whose limit is in characters, which its metadata says
func (m *Model) InputTokenLimit() *uint {
	return nil
}

// OutputTokenLimit isn't known, as a model transcribes audio, or speaks
func (m *Model) OutputTokenLimit() *uint {
	return nil
}

// Capabilities returns what the model can do, which is transcribe speech,
// but not translate it, or speak, nor anything else
func (m *Model) Capabilities() llm.ModelCap {
	yes, no := types.Ptr(true), types.Ptr(false)
	if m.speech != nil {
		return llm.ModelCap{Chat: no, Tools: no, Thinking: no, ImageInput: no, OCR: no, Transcription: no, Translation: no, Embedding: no, SpeechOutput: yes}
	}
	return llm.ModelCap{Chat: no, Tools: no, Thinking: no, ImageInput: no, OCR: no, Transcription: yes, Translation: no, Embedding: no, SpeechOutput: no}
}

///////////////////////////////////////////////////////////////////////////////
// PROVIDER METHODS

// ListModels returns the Scribe models, which transcribe speech, and the
// models ElevenLabs lists which speak
func (c *Client) ListModels(ctx context.Context) ([]llm.Model, error) {
	speech, err := c.speechModels(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]llm.Model, 0, len(models)+len(speech))
	for _, model := range models {
		result = append(result, &Model{provider: c, name: model.name, description: model.description})
	}
	for i := range speech {
		result = append(result, c.speechModel(speech[i]))
	}
	return result, nil
}

// GetModel returns the Scribe model, or the model which speaks, with the
// name, or ErrNotFound
func (c *Client) GetModel(ctx context.Context, name string) (llm.Model, error) {
	for _, model := range models {
		if model.name == name {
			return &Model{provider: c, name: model.name, description: model.description}, nil
		}
	}
	speech, err := c.speechModels(ctx)
	if err != nil {
		return nil, err
	}
	for i := range speech {
		if speech[i].ID == name {
			return c.speechModel(speech[i]), nil
		}
	}
	return nil, llm.ErrNotFound.Withf("model %q", name)
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// speechModel returns the model, which speaks
func (c *Client) speechModel(data apiModel) *Model {
	description := data.Description
	if description == "" {
		description = data.Name
	}
	return &Model{provider: c, name: data.ID, description: description, speech: &data}
}

// speechModels returns the models ElevenLabs lists which speak, which are
// kept for PingInterval, as they seldom change
func (c *Client) speechModels(ctx context.Context) ([]apiModel, error) {
	c.speechMu.Lock()
	defer c.speechMu.Unlock()
	if !c.listed.IsZero() && time.Since(c.listed) < PingInterval {
		return c.speech, nil
	}
	var list []apiModel
	if err := c.DoWithContext(ctx, nil, &list, client.OptPath("models")); err != nil {
		return nil, err
	}
	speech := make([]apiModel, 0, len(list))
	for _, model := range list {
		if model.Speaks && model.ID != "" {
			speech = append(speech, model)
		}
	}
	c.speech, c.listed = speech, time.Now()
	return speech, nil
}

///////////////////////////////////////////////////////////////////////////////
// JSON

// UnmarshalJSON reads the model's fields, and keeps them all
func (m *apiModel) UnmarshalJSON(data []byte) error {
	type model apiModel
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, (*model)(m)); err != nil {
		return err
	} else if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	m.Fields = fields
	return nil
}
