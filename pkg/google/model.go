package google

import (
	"context"
	"strings"

	client "github.com/mutablelogic/go-client"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type listModelsResponse struct {
	Models        []modelResponse `json:"models"`
	NextPageToken string          `json:"nextPageToken,omitempty"`
}

type modelResponse struct {
	Name                       string   `json:"name"`
	BaseModelId                string   `json:"baseModelId,omitempty"`
	Version                    string   `json:"version,omitempty"`
	DisplayName                string   `json:"displayName,omitempty"`
	Description                string   `json:"description,omitempty"`
	InputTokenLimit            int      `json:"inputTokenLimit,omitempty"`
	OutputTokenLimit           int      `json:"outputTokenLimit,omitempty"`
	SupportedGenerationMethods []string `json:"supportedGenerationMethods,omitempty"`
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// ListModels returns the list of available models
func (c *Client) ListModels(ctx context.Context) ([]schema.Model, error) {
	var response listModelsResponse
	if err := c.DoWithContext(ctx, nil, &response, client.OptPath("models")); err != nil {
		return nil, err
	}

	result := make([]schema.Model, 0, len(response.Models))
	for _, m := range response.Models {
		result = append(result, m.toSchema())
	}
	return result, nil
}

// GetModel returns the model with the given name
func (c *Client) GetModel(ctx context.Context, name string) (*schema.Model, error) {
	var response modelResponse
	if err := c.DoWithContext(ctx, nil, &response, client.OptPath("models", name)); err != nil {
		return nil, err
	}

	result := response.toSchema()
	return &result, nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// toSchema converts an API model response to schema.Model
func (m modelResponse) toSchema() schema.Model {
	description := m.Description
	if description == "" {
		description = m.DisplayName
	}
	return schema.Model{
		Name:        strings.TrimPrefix(m.Name, "models/"),
		Description: description,
		OwnedBy:     defaultName,
	}
}
