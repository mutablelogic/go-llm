package mistral

import (
	"context"
	"slices"
	"time"

	// Packages
	client "github.com/mutablelogic/go-client"
	"github.com/mutablelogic/go-llm/pkg/opt"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	"github.com/mutablelogic/go-llm/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type modelResponse struct {
	ID                      string          `json:"id"`
	Object                  string          `json:"object"`
	OwnedBy                 string          `json:"owned_by"`
	Name                    string          `json:"name"`
	Description             string          `json:"description,omitempty"`
	Created                 int64           `json:"created,omitempty"`
	Deprecated              time.Time       `json:"deprecated,omitempty"`
	Alternative             string          `json:"deprecation_replacement_model,omitempty"`
	MaxContextLength        int64           `json:"max_context_length,omitempty"`
	Capabilities            map[string]bool `json:"capabilities,omitempty"`
	Aliases                 []string        `json:"aliases,omitempty"`
	DefaultModelTemperature float64         `json:"default_model_temperature,omitempty"`
}

type listModelsResponse struct {
	Data []modelResponse `json:"data"`
}

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
		models := make([]schema.Model, 0, len(response.Data))
		for _, m := range response.Data {
			models = append(models, m.toSchema())
		}

		// Return the models
		return models, nil
	})
}

// GetModel returns a specific model by name or ID
func (c *Client) GetModel(ctx context.Context, name string) (*schema.Model, error) {
	return c.ModelCache.GetModel(ctx, name, func(ctx context.Context, name string) (*schema.Model, error) {
		var response modelResponse
		if err := c.DoWithContext(ctx, nil, &response, client.OptPath("models", name)); err != nil {
			return nil, err
		}
		return types.Ptr(response.toSchema()), nil
	})
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

func (m modelResponse) toSchema() schema.Model {

	// Convert created time
	created := time.Unix(m.Created, 0)

	// Add fields to metadata
	meta := make(map[string]any)
	if m.MaxContextLength != 0 {
		meta["max_context_length"] = m.MaxContextLength
	}
	if m.DefaultModelTemperature != 0 {
		meta["default_model_temperature"] = m.DefaultModelTemperature
	}

	// Include capabilities
	meta["capabilities"] = m.Capabilities

	// Include deprecation info
	if !m.Deprecated.IsZero() {
		meta["replacement_model"] = m.Alternative
	}

	// Append the name onto the aliases if not already present
	if m.Name != "" && !slices.Contains(m.Aliases, m.Name) {
		m.Aliases = append(m.Aliases, m.Name)
	}

	// Retunrn the model
	return schema.Model{
		Name:        m.ID,
		Description: m.Description,
		Created:     created,
		OwnedBy:     defaultName,
		Aliases:     m.Aliases,
		Meta:        meta,
	}
}
