package gemini

import (
	"context"
	"encoding/json"

	// Packages

	"github.com/mutablelogic/go-llm"
	"google.golang.org/genai"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type model struct {
	*Client `json:"-"`
	meta    *genai.Model
}

var _ llm.Model = (*model)(nil)

///////////////////////////////////////////////////////////////////////////////
// STRINGIFY

func (m model) MarshalJSON() ([]byte, error) {
	return json.Marshal(m.meta)
}

func (m model) String() string {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err.Error()
	}
	return string(data)
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS - llm.Model implementation

// Return model name
func (m model) Name() string {
	return m.meta.Name
}

// Return model aliases
func (model model) Aliases() []string {
	return nil
}

// Return model description
func (model model) Description() string {
	return model.meta.Description
}

// Return the models
func (gemini *Client) Models(ctx context.Context) ([]llm.Model, error) {
	// Cache models
	if gemini.cache == nil {
		gemini.cache = make(map[string]llm.Model)
		for m := range gemini.Client.Models.All(ctx) {
			gemini.cache[m.Name] = &model{gemini, m}
		}
	}

	// Return models
	result := make([]llm.Model, 0, len(gemini.cache))
	for _, model := range gemini.cache {
		result = append(result, model)
	}
	return result, nil
}

// Return a model by name, or nil if not found.
// Panics on error.
func (gemini *Client) Model(ctx context.Context, name string) llm.Model {
	if gemini.cache == nil {
		if _, err := gemini.Models(ctx); err != nil {
			panic(err)
		}
	}
	return gemini.cache[name]
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS - MODEL

// Return an empty session context object for the model,
// setting session options
func (m model) Context(...llm.Opt) llm.Context {
	return nil
}

// Create a completion from a text prompt
func (m model) Completion(context.Context, string, ...llm.Opt) (llm.Completion, error) {
	return nil, llm.ErrNotImplemented
}

// Embedding vector generation
func (m model) Embedding(context.Context, string, ...llm.Opt) ([]float64, error) {
	return nil, llm.ErrNotImplemented
}
