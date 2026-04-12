package openai

import (
	"context"
	"time"

	// Packages
	client "github.com/mutablelogic/go-client"
	schema "github.com/mutablelogic/go-llm/kernel/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type model struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	OwnedBy string `json:"owned_by"`
}

type listModelsResponse struct {
	Object string  `json:"object"`
	Data   []model `json:"data"`
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// ListModels returns the list of available models
func (c *Client) ListModels(ctx context.Context) ([]schema.Model, error) {
	// Get models
	var response listModelsResponse
	if err := c.DoWithContext(ctx, nil, &response, client.OptPath("models")); err != nil {
		return nil, err
	}

	// Return models
	models := make([]schema.Model, len(response.Data))
	for i, m := range response.Data {
		models[i] = m.toSchema()
	}
	return models, nil
}

// GetModel returns the model with the given name
func (c *Client) GetModel(ctx context.Context, name string) (*schema.Model, error) {
	// Get model
	var response model
	if err := c.DoWithContext(ctx, nil, &response, client.OptPath("models", name)); err != nil {
		return nil, err
	}

	// Return model
	return types.Ptr(response.toSchema()), nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

func (m model) toSchema() schema.Model {
	return schema.Model{
		Name:        m.ID,
		Description: m.ID,
		Created:     time.Unix(m.Created, 0),
		OwnedBy:     m.OwnedBy,
	}
}
