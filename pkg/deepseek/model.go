package deepseek

import (
	"context"
	"encoding/json"

	// Packages
	"github.com/mutablelogic/go-client"
	"github.com/mutablelogic/go-llm"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type model struct {
	*Client `json:"-"`
	meta    Model
}

var _ llm.Model = (*model)(nil)

type Model struct {
	Name    string `json:"id"`
	Type    string `json:"object"`
	OwnedBy string `json:"owned_by"`
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
func (deepseek *Client) Models(ctx context.Context) ([]llm.Model, error) {
	return deepseek.ModelCache.Load(func() ([]llm.Model, error) {
		return deepseek.loadmodels(ctx)
	})
}

// Return a model by name, or nil if not found.
// Panics on error.
func (deepseek *Client) Model(ctx context.Context, name string) llm.Model {
	model, err := deepseek.ModelCache.Get(func() ([]llm.Model, error) {
		return deepseek.loadmodels(ctx)
	}, name)
	if err != nil {
		panic(err)
	}
	return model
}

// Function called to load models
func (deepseek *Client) loadmodels(ctx context.Context) ([]llm.Model, error) {
	if models, err := deepseek.ListModels(ctx); err != nil {
		return nil, err
	} else {
		result := make([]llm.Model, len(models))
		for i, meta := range models {
			result[i] = &model{deepseek, meta}
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

// Return model aliases
func (model model) Aliases() []string {
	return nil
}

// Return model description
func (model model) Description() string {
	return ""
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS - API

// ListModels returns all the models
func (deepseek *Client) ListModels(ctx context.Context) ([]Model, error) {
	// Response
	var response struct {
		Data []Model `json:"data"`
	}
	if err := deepseek.DoWithContext(ctx, nil, &response, client.OptPath("models")); err != nil {
		return nil, err
	}

	// Return success
	return response.Data, nil
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS - MODEL

// Return a new empty session
func (model *model) Context(opts ...llm.Opt) llm.Context {
	return nil
	// return impl.NewSession(model, &messagefactory{}, opts...)
}

// Create a completion from a text prompt
func (model *model) Completion(context.Context, string, ...llm.Opt) (llm.Completion, error) {
	return nil, llm.ErrNotImplemented
}

// Create a completion from a chat session
func (model *model) Chat(context.Context, []llm.Completion, ...llm.Opt) (llm.Completion, error) {
	return nil, llm.ErrNotImplemented
}

// Embedding vector generation
func (model *model) Embedding(context.Context, string, ...llm.Opt) ([]float64, error) {
	return nil, llm.ErrNotImplemented
}
