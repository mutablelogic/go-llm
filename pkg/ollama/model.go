package ollama

import (
	"context"
	"time"

	// Packages
	client "github.com/mutablelogic/go-client"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// model represents the API response for a model from Ollama
type model struct {
	Name       string       `json:"name"`
	Model      string       `json:"model,omitempty"`
	ModifiedAt time.Time    `json:"modified_at"`
	Size       int64        `json:"size,omitempty"`
	Digest     string       `json:"digest,omitempty"`
	Details    ModelDetails `json:"details"`
	File       string       `json:"modelfile,omitempty"`
	Parameters string       `json:"parameters,omitempty"`
	Template   string       `json:"template,omitempty"`
	Info       ModelInfo    `json:"model_info,omitempty"`
}

// ModelDetails are the details of the model
type ModelDetails struct {
	ParentModel       string   `json:"parent_model,omitempty"`
	Format            string   `json:"format"`
	Family            string   `json:"family"`
	Families          []string `json:"families"`
	ParameterSize     string   `json:"parameter_size"`
	QuantizationLevel string   `json:"quantization_level"`
}

// ModelInfo provides additional model parameters
type ModelInfo map[string]any

// listModelsResponse represents the API response for listing models
type listModelsResponse struct {
	Data []model `json:"models"`
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// List all models in the Ollama registry
func (ollama *Client) ListModels(ctx context.Context) ([]schema.Model, error) {
	// Send the request
	var response listModelsResponse
	if err := ollama.DoWithContext(ctx, nil, &response, client.OptPath("tags")); err != nil {
		return nil, err
	}

	result := make([]schema.Model, len(response.Data))
	for i, m := range response.Data {
		result[i] = m.toSchema()
	}

	// Return models
	return result, nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// toSchema converts an API model response to schema.Model
func (m model) toSchema() schema.Model {
	return schema.Model{
		Name:        m.Name,
		Description: m.Model,
		Created:     m.ModifiedAt,
		OwnedBy:     defaultName,
	}
}
