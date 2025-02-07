package anthropic

import (
	"context"
	"encoding/json"
	"net/url"
	"time"

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
// PUBLIC METHODS - llm.Agent

// Return the models
func (anthropic *Client) Models(ctx context.Context) ([]llm.Model, error) {
	return anthropic.ModelCache.Load(func() ([]llm.Model, error) {
		return anthropic.loadmodels(ctx)
	})
}

// Return a model by name, or nil if not found.
// Panics on error.
func (anthropic *Client) Model(ctx context.Context, name string) llm.Model {
	model, err := anthropic.ModelCache.Get(func() ([]llm.Model, error) {
		return anthropic.loadmodels(ctx)
	}, name)
	if err != nil {
		panic(err)
	}
	return model
}

// Function called to load models
func (anthropic *Client) loadmodels(ctx context.Context) ([]llm.Model, error) {
	if models, err := anthropic.ListModels(ctx); err != nil {
		return nil, err
	} else {
		result := make([]llm.Model, len(models))
		for i, meta := range models {
			result[i] = &model{anthropic, meta}
		}
		return result, nil
	}
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS - llm.Model

// Return the name of a model
func (model *model) Name() string {
	return model.meta.Name
}

// Return model description
func (model model) Description() string {
	return model.meta.Description
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
// PUBLIC METHODS - API

// List models
func (anthropic *Client) ListModels(ctx context.Context) ([]Model, error) {
	var response struct {
		Body    []Model `json:"data"`
		HasMore bool    `json:"has_more"`
		FirstId string  `json:"first_id"`
		LastId  string  `json:"last_id"`
	}

	// Request
	request := url.Values{}
	result := make([]Model, 0, 100)
	for {
		if err := anthropic.DoWithContext(ctx, nil, &response, client.OptPath("models"), client.OptQuery(request)); err != nil {
			return nil, err
		}

		// Convert to llm.Model
		for _, meta := range response.Body {
			result = append(result, meta)
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

// Get a model by name
func (anthropic *Client) GetModel(ctx context.Context, name string) (*Model, error) {
	var response Model
	if err := anthropic.DoWithContext(ctx, nil, &response, client.OptPath("models", name)); err != nil {
		return nil, err
	}

	// Return success
	return &response, nil
}

// Embedding vector generation - not supported on Anthropic
func (*model) Embedding(context.Context, string, ...llm.Opt) ([]float64, error) {
	return nil, llm.ErrNotImplemented
}
