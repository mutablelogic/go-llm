package mistral

import (
	"context"
	"encoding/json"

	"github.com/mutablelogic/go-client"
	"github.com/mutablelogic/go-llm"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type model struct {
	meta Model
}

type Model struct {
	Name                    string   `json:"id"`
	Description             string   `json:"description,omitempty"`
	Type                    string   `json:"type,omitempty"`
	CreatedAt               *uint64  `json:"created,omitempty"`
	OwnedBy                 string   `json:"owned_by,omitempty"`
	MaxContextLength        uint64   `json:"max_context_length,omitempty"`
	Aliases                 []string `json:"aliases,omitempty"`
	Deprecation             *string  `json:"deprecation,omitempty"`
	DefaultModelTemperature *float64 `json:"default_model_temperature,omitempty"`
	Capabilities            struct {
		CompletionChat  bool `json:"completion_chat,omitempty"`
		CompletionFim   bool `json:"completion_fim,omitempty"`
		FunctionCalling bool `json:"function_calling,omitempty"`
		FineTuning      bool `json:"fine_tuning,omitempty"`
		Vision          bool `json:"vision,omitempty"`
	} `json:"capabilities,omitempty"`
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
// PUBLIC METHODS - API

// ListModels returns all the models
func (c *Client) ListModels(ctx context.Context) ([]llm.Model, error) {
	// Response
	var response struct {
		Data []Model `json:"data"`
	}
	if err := c.DoWithContext(ctx, nil, &response, client.OptPath("models")); err != nil {
		return nil, err
	}

	//  Make models
	result := make([]llm.Model, 0, len(response.Data))
	for _, meta := range response.Data {
		result = append(result, &model{meta: meta})
	}

	// Return models
	return result, nil
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS - MODEL

// Return the name of the model
func (m model) Name() string {
	return m.meta.Name
}

// Embedding vector generation
func (m model) Embedding(context.Context, string, ...llm.Opt) ([]float64, error) {
	return nil, llm.ErrNotImplemented
}
