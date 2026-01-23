package mistral

import (
	"context"
	"time"

	// Packages
	client "github.com/mutablelogic/go-client"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type modelResponse struct {
	ID          string `json:"id"`
	Object      string `json:"object"`
	OwnedBy     string `json:"owned_by"`
	Description string `json:"description,omitempty"`
	Created     int64  `json:"created,omitempty"`
}

type listModelsResponse struct {
	Data []modelResponse `json:"data"`
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// ListModels returns all available models from the Mistral API
func (c *Client) ListModels(ctx context.Context) ([]schema.Model, error) {
	var response listModelsResponse
	if err := c.DoWithContext(ctx, nil, &response, client.OptPath("models")); err != nil {
		return nil, err
	}

	models := make([]schema.Model, 0, len(response.Data))
	for _, m := range response.Data {
		models = append(models, m.toSchema())
	}
	return models, nil
}

// GetModel returns a specific model by name or ID
func (c *Client) GetModel(ctx context.Context, name string) (*schema.Model, error) {
	var response modelResponse
	if err := c.DoWithContext(ctx, nil, &response, client.OptPath("models", name)); err != nil {
		return nil, err
	}
	model := response.toSchema()
	return &model, nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

func (m modelResponse) toSchema() schema.Model {
	created := time.Unix(m.Created, 0)
	return schema.Model{
		Name:        m.ID,
		Description: m.Description,
		Created:     created,
		OwnedBy:     defaultName,
	}
}
