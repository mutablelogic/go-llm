package anthropic

import (
	"context"
	"net/url"
	"time"

	// Packages
	client "github.com/mutablelogic/go-client"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// model represents the API response for a model from Anthropic
type model struct {
	Id          string    `json:"id"`
	DisplayName string    `json:"display_name"`
	Type        string    `json:"type"`
	CreatedAt   time.Time `json:"created_at"`
}

// listModelsResponse represents the API response for listing models
type listModelsResponse struct {
	Data    []model `json:"data"`
	HasMore bool    `json:"has_more"`
	FirstId string  `json:"first_id"`
	LastId  string  `json:"last_id"`
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// ListModels returns all available models from the Anthropic API
func (anthropic *Client) ListModels(ctx context.Context) ([]schema.Model, error) {
	var response listModelsResponse

	// Request with pagination
	request := url.Values{}
	result := make([]schema.Model, 0, 100)
	for {
		if err := anthropic.DoWithContext(ctx, nil, &response, client.OptPath("models"), client.OptQuery(request)); err != nil {
			return nil, err
		}

		// Convert to schema.Model
		for _, m := range response.Data {
			result = append(result, m.toSchema())
		}

		// If there are no more models, return
		if !response.HasMore {
			break
		}
		request.Set("after_id", response.LastId)
	}

	// Return models
	return result, nil
}

// GetModel returns a specific model by name or ID
func (anthropic *Client) GetModel(ctx context.Context, name string) (*schema.Model, error) {
	var response model
	if err := anthropic.DoWithContext(ctx, nil, &response, client.OptPath("models", name)); err != nil {
		return nil, err
	}

	// Return success
	result := response.toSchema()
	return &result, nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// toSchema converts an API model response to schema.Model
func (m model) toSchema() schema.Model {
	return schema.Model{
		Name:        m.Id,
		Description: m.DisplayName,
		Created:     m.CreatedAt,
		OwnedBy:     defaultName,
	}
}
