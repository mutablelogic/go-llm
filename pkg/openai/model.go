package openai

import (
	"context"
	"encoding/json"
	"fmt"

	// Packages
	client "github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
	impl "github.com/mutablelogic/go-llm/pkg/internal/impl"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type model struct {
	*Client `json:"-"`
	meta    Model
}

var _ llm.Model = (*model)(nil)

type Model struct {
	Name      string `json:"id"`
	Type      string `json:"object,omitempty"`
	CreatedAt uint64 `json:"created,omitempty"`
	OwnedBy   string `json:"owned_by,omitempty"`
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
// PUBLIC METHODS - llm.Agent

// Return the models
func (openai *Client) Models(ctx context.Context) ([]llm.Model, error) {
	return openai.ModelCache.Load(func() ([]llm.Model, error) {
		return openai.loadmodels(ctx)
	})
}

// Return a model by name, or nil if not found.
// Panics on error.
func (openai *Client) Model(ctx context.Context, name string) llm.Model {
	model, err := openai.ModelCache.Get(func() ([]llm.Model, error) {
		return openai.loadmodels(ctx)
	}, name)
	if err != nil {
		panic(err)
	}
	return model
}

// Function called to load models
func (openai *Client) loadmodels(ctx context.Context) ([]llm.Model, error) {
	if models, err := openai.ListModels(ctx); err != nil {
		return nil, err
	} else {
		result := make([]llm.Model, len(models))
		for i, meta := range models {
			result[i] = &model{openai, meta}
		}
		return result, nil
	}
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS - llm.Model

// Return model name
func (model model) Name() string {
	return model.meta.Name
}

// Return model description
func (model model) Description() string {
	return fmt.Sprintf("Owner: %q", model.meta.OwnedBy)
}

// Return model aliases
func (model) Aliases() []string {
	return nil
}

// Return a new empty session
func (model *model) Context(opts ...llm.Opt) llm.Context {
	return impl.NewSession(model, &messagefactory{}, opts...)
}

///////////////////////////////////////////////////////////////////////////////
// API CALLS

// ListModels returns all the models
func (openai *Client) ListModels(ctx context.Context) ([]Model, error) {
	// Return the response
	var response struct {
		Data []Model `json:"data"`
	}
	if err := openai.DoWithContext(ctx, nil, &response, client.OptPath("models")); err != nil {
		return nil, err
	}

	// Return success
	return response.Data, nil
}

// GetModel returns one model
func (openai *Client) GetModel(ctx context.Context, model string) (*Model, error) {
	// Return the response
	var response Model
	if err := openai.DoWithContext(ctx, nil, &response, client.OptPath("models", model)); err != nil {
		return nil, err
	}

	// Return success
	return &response, nil
}

// Delete a fine-tuned model. You must have the Owner role in your organization
// to delete a model.
func (openai *Client) DeleteModel(ctx context.Context, model string) error {
	return openai.DoWithContext(ctx, client.MethodDelete, nil, client.OptPath("models", model))
}
