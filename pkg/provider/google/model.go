package google

import (
	"context"
	"encoding/json"
	"net/url"
	"strings"

	// Packages
	client "github.com/mutablelogic/go-client"
	opt "github.com/mutablelogic/go-llm/pkg/opt"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	types "github.com/mutablelogic/go-llm/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// ListModels returns all available models from the Gemini API
func (c *Client) ListModels(ctx context.Context, opts ...opt.Opt) ([]schema.Model, error) {
	return c.ModelCache.ListModels(ctx, opts, func(ctx context.Context, opts ...opt.Opt) ([]schema.Model, error) {
		var response geminiListModelsResponse

		// Request with pagination
		request := url.Values{}
		result := make([]schema.Model, 0, 100)
		for {
			if err := c.DoWithContext(ctx, nil, &response, client.OptPath("models"), client.OptQuery(request)); err != nil {
				return nil, err
			}

			// Convert to schema.Model
			for _, m := range response.Models {
				result = append(result, m.toSchema())
			}

			// If there are no more models, return
			if response.NextPageToken == "" {
				break
			}
			request.Set("pageToken", response.NextPageToken)
		}

		// Return models
		return result, nil
	})
}

// GetModel returns a specific model by name
func (c *Client) GetModel(ctx context.Context, name string, opts ...opt.Opt) (*schema.Model, error) {
	return c.ModelCache.GetModel(ctx, name, func(ctx context.Context, name string) (*schema.Model, error) {
		var response geminiModel
		if err := c.DoWithContext(ctx, nil, &response, client.OptPath("models", name)); err != nil {
			return nil, err
		}
		return types.Ptr(response.toSchema()), nil
	})
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// toSchema converts a geminiModel wire type to schema.Model
func (m *geminiModel) toSchema() schema.Model {
	description := m.Description
	if description == "" {
		description = m.DisplayName
	}

	// JSON round-trip to capture all fields as map[string]any
	var meta map[string]any
	if data, err := json.Marshal(m); err == nil {
		json.Unmarshal(data, &meta)
	}

	// Return the model
	return schema.Model{
		Name:        strings.TrimPrefix(m.Name, "models/"),
		Description: description,
		OwnedBy:     defaultName,
		Meta:        meta,
	}
}
