package mistral

import (
	"context"
	"encoding/json"

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
	Name                    string   `json:"id"`
	Description             string   `json:"description,omitempty"`
	Type                    string   `json:"type,omitempty"`
	CreatedAt               *uint64  `json:"created,omitempty"`
	OwnedBy                 string   `json:"owned_by,omitempty"`
	MaxContextLength        uint64   `json:"max_context_length,omitempty"`
	Aliases                 []string `json:"aliases,omitempty"`
	Deprecation             *string  `json:"deprecation,omitempty"`
	DefaultModelTemperature *float64 `json:"default_model_temperature,omitempty"`
	Capabilities            struct {
		CompletionChat  bool `json:"completion_chat,omitempty"`
		CompletionFim   bool `json:"completion_fim,omitempty"`
		FunctionCalling bool `json:"function_calling,omitempty"`
		FineTuning      bool `json:"fine_tuning,omitempty"`
		Vision          bool `json:"vision,omitempty"`
	} `json:"capabilities,omitempty"`
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
func (mistral *Client) Models(ctx context.Context) ([]llm.Model, error) {
	return mistral.ModelCache.Load(func() ([]llm.Model, error) {
		return mistral.loadmodels(ctx)
	})
}

// Return a model by name, or nil if not found.
// Panics on error.
func (mistral *Client) Model(ctx context.Context, name string) llm.Model {
	model, err := mistral.ModelCache.Get(func() ([]llm.Model, error) {
		return mistral.loadmodels(ctx)
	}, name)
	if err != nil {
		panic(err)
	}
	return model
}

// Function called to load models
func (mistral *Client) loadmodels(ctx context.Context) ([]llm.Model, error) {
	if models, err := mistral.ListModels(ctx); err != nil {
		return nil, err
	} else {
		result := make([]llm.Model, len(models))
		for i, meta := range models {
			result[i] = &model{mistral, meta}
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
	return model.meta.Aliases
}

// Return model description
func (model model) Description() string {
	return model.meta.Description
}

// Return a new empty session
func (model *model) Context(opts ...llm.Opt) llm.Context {
	return impl.NewSession(model, &messagefactory{}, opts...)
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS - API

// ListModels returns all the models
func (mistral *Client) ListModels(ctx context.Context) ([]Model, error) {
	// Response
	var response struct {
		Data []Model `json:"data"`
	}
	if err := mistral.DoWithContext(ctx, nil, &response, client.OptPath("models")); err != nil {
		return nil, err
	}

	// Return success
	return response.Data, nil
}

// GetModel returns one model
func (mistral *Client) GetModel(ctx context.Context, model string) (*Model, error) {
	// Return the response
	var response Model
	if err := mistral.DoWithContext(ctx, nil, &response, client.OptPath("models", model)); err != nil {
		return nil, err
	}

	// Return success
	return &response, nil
}

// Delete a fine-tuned model
func (c *Client) DeleteModel(ctx context.Context, model string) error {
	if err := c.DoWithContext(ctx, client.MethodDelete, nil, client.OptPath("models", model)); err != nil {
		return err
	}

	// Return success
	return nil
}
