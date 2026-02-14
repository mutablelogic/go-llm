package mistral

import (
	"context"
	"encoding/json"
	"time"

	// Packages
	client "github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
	opt "github.com/mutablelogic/go-llm/pkg/opt"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// ListModels returns all available models from the Mistral API
func (c *Client) ListModels(ctx context.Context, opts ...opt.Opt) ([]schema.Model, error) {
	return c.ModelCache.ListModels(ctx, opts, func(ctx context.Context, opts ...opt.Opt) ([]schema.Model, error) {
		var response listModelsResponse

		if err := c.DoWithContext(ctx, nil, &response, client.OptPath("models")); err != nil {
			return nil, err
		}

		// Convert to schema.Model
		result := make([]schema.Model, 0, len(response.Data))
		for _, m := range response.Data {
			result = append(result, m.toSchema())
		}

		return result, nil
	})
}

// GetModel returns a specific model by name
func (c *Client) GetModel(ctx context.Context, name string, opts ...opt.Opt) (*schema.Model, error) {
	return c.ModelCache.GetModel(ctx, name, func(ctx context.Context, name string) (*schema.Model, error) {
		// Mistral doesn't have a single-model endpoint, so list and find
		models, err := c.ListModels(ctx, opts...)
		if err != nil {
			return nil, err
		}
		for _, m := range models {
			if m.Name == name {
				return types.Ptr(m), nil
			}
		}
		return nil, llm.ErrNotFound.Withf("model not found: %s", name)
	})
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE TYPES

// capabilities represents the model capabilities from the Mistral API
type capabilities struct {
	CompletionChat     bool `json:"completion_chat"`
	FunctionCalling    bool `json:"function_calling"`
	CompletionFim      bool `json:"completion_fim"`
	FineTuning         bool `json:"fine_tuning"`
	Vision             bool `json:"vision"`
	OCR                bool `json:"ocr"`
	Classification     bool `json:"classification"`
	Moderation         bool `json:"moderation"`
	Audio              bool `json:"audio"`
	AudioTranscription bool `json:"audio_transcription"`
}

// model represents a model from the Mistral API response
type model struct {
	Id                          string       `json:"id"`
	Object                      string       `json:"object"`
	Created                     int64        `json:"created"`
	OwnedBy                     string       `json:"owned_by"`
	Name                        string       `json:"name,omitempty"`
	Description                 string       `json:"description,omitempty"`
	Capabilities                capabilities `json:"capabilities"`
	MaxContextLength            int          `json:"max_context_length,omitempty"`
	Aliases                     []string     `json:"aliases,omitempty"`
	Deprecation                 string       `json:"deprecation,omitempty"`
	DeprecationReplacementModel string       `json:"deprecation_replacement_model,omitempty"`
	DefaultModelTemperature     *float64     `json:"default_model_temperature,omitempty"`
	Type                        string       `json:"type,omitempty"`
}

// listModelsResponse is the response from GET /v1/models
type listModelsResponse struct {
	Object string  `json:"object"`
	Data   []model `json:"data"`
}

// toSchema converts a Mistral model to schema.Model
func (m model) toSchema() schema.Model {
	// JSON round-trip to capture all fields as meta
	var meta map[string]any
	if data, err := json.Marshal(m); err == nil {
		json.Unmarshal(data, &meta)
	}

	return schema.Model{
		Name:        m.Id,
		Description: m.Description,
		Created:     time.Unix(m.Created, 0),
		OwnedBy:     schema.Mistral,
		Aliases:     m.Aliases,
		Meta:        meta,
	}
}
