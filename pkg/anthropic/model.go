package anthropic

import (
	"context"
	"encoding/json"
	"net/url"
	"time"

	// Packages
	client "github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type model struct {
	*Client `json:"-"`
	meta    Model
}

var _ llm.Model = (*model)(nil)

type Model struct {
	Name        string     `json:"id"`
	Description string     `json:"display_name,omitempty"`
	Type        string     `json:"type,omitempty"`
	CreatedAt   *time.Time `json:"created_at,omitempty"`
}

///////////////////////////////////////////////////////////////////////////////
// STRINGIFY

func (m model) MarshalJSON() ([]byte, error) {
	return json.Marshal(m.meta)
}

func (m model) String() string {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err.Error()
	}
	return string(data)
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS - API

// Get a model by name
func (anthropic *Client) GetModel(ctx context.Context, name string) (llm.Model, error) {
	var response Model
	if err := anthropic.DoWithContext(ctx, nil, &response, client.OptPath("models", name)); err != nil {
		return nil, err
	}

	// Return success
	return &model{anthropic, response}, nil
}

// List models
func (anthropic *Client) ListModels(ctx context.Context) ([]llm.Model, error) {
	var response struct {
		Body    []Model `json:"data"`
		HasMore bool    `json:"has_more"`
		FirstId string  `json:"first_id"`
		LastId  string  `json:"last_id"`
	}

	// Request
	request := url.Values{}
	result := make([]llm.Model, 0, 100)
	for {
		if err := anthropic.DoWithContext(ctx, nil, &response, client.OptPath("models"), client.OptQuery(request)); err != nil {
			return nil, err
		}

		// Convert to llm.Model
		for _, meta := range response.Body {
			result = append(result, &model{anthropic, meta})
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
	return model.meta.Name
}

// Embedding vector generation - not supported on Anthropic
func (*model) Embedding(context.Context, string, ...llm.Opt) ([]float64, error) {
	return nil, llm.ErrNotImplemented
}
