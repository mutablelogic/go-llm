package anthropic

import (
	"context"
	"net/url"
	"time"

	// Packages
	client "github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// model is the implementation of the llm.Model interface
type model struct {
	ModelMeta
}

var _ llm.Model = (*model)(nil)

// ModelMeta is the metadata for an anthropic model
type ModelMeta struct {
	Name        string     `json:"id"`
	Description string     `json:"display_name,omitempty"`
	Type        string     `json:"type,omitempty"`
	CreatedAt   *time.Time `json:"created_at,omitempty"`
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Agent interface
func (anthropic *Client) Models(ctx context.Context) ([]llm.Model, error) {
	return anthropic.ListModels(ctx)
}

// Get a model by name
func (anthropic *Client) GetModel(ctx context.Context, name string) (llm.Model, error) {

	var response ModelMeta
	if err := anthropic.DoWithContext(ctx, nil, &response, client.OptPath("models", name)); err != nil {
		return nil, err
	}

	// Return success
	return &model{ModelMeta: response}, nil
}

// List models
func (anthropic *Client) ListModels(ctx context.Context) ([]llm.Model, error) {
	// Send the request
	var response struct {
		Body    []ModelMeta `json:"data"`
		HasMore bool        `json:"has_more"`
		FirstId string      `json:"first_id"`
		LastId  string      `json:"last_id"`
	}

	request := url.Values{}
	result := make([]llm.Model, 0, 100)
	for {
		if err := anthropic.DoWithContext(ctx, nil, &response, client.OptPath("models"), client.OptQuery(request)); err != nil {
			return nil, err
		}

		// Convert to llm.Model
		for _, meta := range response.Body {
			result = append(result, &model{
				ModelMeta: meta,
			})
		}

		// If there are no more models, return
		if !response.HasMore {
			break
		} else {
			request.Set("after_id", response.LastId)
		}
	}

	// Return models
	return result, nil
}

// Return the name of a model
func (model *model) Name() string {
	return model.ModelMeta.Name
}
