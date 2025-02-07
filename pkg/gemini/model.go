package gemini

import (
	"context"
	"encoding/json"

	// Packages
	"github.com/mutablelogic/go-client"
	"github.com/mutablelogic/go-llm"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type model struct {
	*Client `json:"-"`
	meta    Model
}

var _ llm.Model = (*model)(nil)

type Model struct {
	Name                       string   `json:"name"`
	Version                    string   `json:"version"`
	DisplayName                string   `json:"displayName"`
	Description                string   `json:"description"`
	InputTokenLimit            uint64   `json:"inputTokenLimit"`
	OutputTokenLimit           uint64   `json:"outputTokenLimit"`
	SupportedGenerationMethods []string `json:"supportedGenerationMethods"`
	Temperature                float64  `json:"temperature"`
	TopP                       float64  `json:"topP"`
	TopK                       uint64   `json:"topK"`
}

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

// Return the models
func (gemini *Client) Models(ctx context.Context) ([]llm.Model, error) {
	// Cache models
	if gemini.cache == nil {
		models, err := gemini.ListModels(ctx)
		if err != nil {
			return nil, err
		}
		gemini.cache = make(map[string]llm.Model, len(models))
		for _, m := range models {
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
// PUBLIC METHODS - API

// ListModels returns all the models
func (c *Client) ListModels(ctx context.Context) ([]Model, error) {
	// Response
	var response struct {
		Data []Model `json:"models"`
	}
	if err := c.DoWithContext(ctx, nil, &response, client.OptPath("models")); err != nil {
		return nil, err
	}

	// Return success
	return response.Data, nil
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS - MODEL

// Return am empty session context object for the model,
// setting session options
func (m model) Context(...llm.Opt) llm.Context {
	return nil
}

// Create a completion from a text prompt
func (m model) Completion(context.Context, string, ...llm.Opt) (llm.Completion, error) {
	return nil, llm.ErrNotImplemented
}

// Create a completion from a chat session
func (m model) Chat(context.Context, []llm.Completion, ...llm.Opt) (llm.Completion, error) {
	return nil, llm.ErrNotImplemented
}

// Embedding vector generation
func (m model) Embedding(context.Context, string, ...llm.Opt) ([]float64, error) {
	return nil, llm.ErrNotImplemented
}
