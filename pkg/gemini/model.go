package gemini

import (
	"context"
	"strings"

	// Packages
	client "github.com/mutablelogic/go-client"
	opt "github.com/mutablelogic/go-llm/pkg/opt"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	types "github.com/mutablelogic/go-llm/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type listModelsResponse struct {
	Models        []model `json:"models"`
	NextPageToken string  `json:"nextPageToken,omitempty"`
}

type model struct {
	Name                       string   `json:"name"`
	BaseModelId                string   `json:"baseModelId,omitempty"`
	Version                    string   `json:"version,omitempty"`
	DisplayName                string   `json:"displayName,omitempty"`
	Description                string   `json:"description,omitempty"`
	InputTokenLimit            int      `json:"inputTokenLimit,omitempty"`
	OutputTokenLimit           int      `json:"outputTokenLimit,omitempty"`
	SupportedGenerationMethods []string `json:"supportedGenerationMethods,omitempty"`
	DefaultModelTemperature    float64  `json:"temperature,omitempty"`
	DefaultTopP                float64  `json:"topP,omitempty"`
	DefaultTopK                int      `json:"topK,omitempty"`
	MaxModelTemperature        float64  `json:"maxModelTemperature,omitempty"`
	Thinking                   bool     `json:"thinking,omitempty"`
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// ListModels returns the list of available models
func (c *Client) ListModels(ctx context.Context, opts ...opt.Opt) ([]schema.Model, error) {
	return c.ModelCache.ListModels(ctx, opts, func(ctx context.Context, opts ...opt.Opt) ([]schema.Model, error) {
		var response listModelsResponse
		if err := c.DoWithContext(ctx, nil, &response, client.OptPath("models")); err != nil {
			return nil, err
		}

		result := make([]schema.Model, 0, len(response.Models))
		for _, m := range response.Models {
			result = append(result, m.toSchema())
		}
		return result, nil
	})
}

// GetModel returns the model with the given name
func (c *Client) GetModel(ctx context.Context, name string) (*schema.Model, error) {
	return c.ModelCache.GetModel(ctx, name, func(ctx context.Context, name string) (*schema.Model, error) {
		var response model
		if err := c.DoWithContext(ctx, nil, &response, client.OptPath("models", name)); err != nil {
			return nil, err
		}
		return types.Ptr(response.toSchema()), nil
	})
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// toSchema converts an API model response to schema.Model
func (m model) toSchema() schema.Model {
	// Set descriptiion as name or display name
	description := m.Description
	if description == "" {
		description = m.DisplayName
	}

	// Add meta field
	meta := make(map[string]any)
	if m.BaseModelId != "" {
		meta["base_model_id"] = m.BaseModelId
	}
	if m.Version != "" {
		meta["version"] = m.Version
	}
	if m.InputTokenLimit != 0 {
		meta["input_token_limit"] = m.InputTokenLimit
	}
	if m.OutputTokenLimit != 0 {
		meta["output_token_limit"] = m.OutputTokenLimit
	}
	if len(m.SupportedGenerationMethods) > 0 {
		meta["capabilities"] = m.SupportedGenerationMethods
	}
	if m.DefaultModelTemperature != 0 {
		meta["default_model_temperature"] = m.DefaultModelTemperature
	}
	if m.DefaultTopP != 0 {
		meta["default_top_p"] = m.DefaultTopP
	}
	if m.DefaultTopK != 0 {
		meta["default_top_k"] = m.DefaultTopK
	}
	if m.MaxModelTemperature != 0 {
		meta["max_model_temperature"] = m.MaxModelTemperature
	}
	if m.Thinking {
		meta["thinking"] = m.Thinking
	}

	return schema.Model{
		Name:        strings.TrimPrefix(m.Name, "models/"),
		Description: description,
		OwnedBy:     defaultName,
		Meta:        meta,
	}
}
