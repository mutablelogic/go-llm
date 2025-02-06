package openai

import (
	"context"
	"encoding/json"

	// Packages
	client "github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
	session "github.com/mutablelogic/go-llm/pkg/session"
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
// PUBLIC METHODS - llm.Model implementation

// Return model name
func (m model) Name() string {
	return m.meta.Name
}

// Return the models
func (openai *Client) Models(ctx context.Context) ([]llm.Model, error) {
	// Cache models
	if openai.cache == nil {
		models, err := openai.ListModels(ctx)
		if err != nil {
			return nil, err
		}
		openai.cache = make(map[string]llm.Model, len(models))
		for _, m := range models {
			openai.cache[m.Name] = &model{openai, m}
		}
	}

	// Return models
	result := make([]llm.Model, 0, len(openai.cache))
	for _, model := range openai.cache {
		result = append(result, model)
	}
	return result, nil
}

// Return a model by name, or nil if not found.
// Panics on error.
func (openai *Client) Model(ctx context.Context, name string) llm.Model {
	if openai.cache == nil {
		if _, err := openai.Models(ctx); err != nil {
			panic(err)
		}
	}
	return openai.cache[name]
}

// Return a new empty session
func (model *model) Context(opts ...llm.Opt) llm.Context {
	return session.NewSession(model, &messagefactory{}, opts...)
}

///////////////////////////////////////////////////////////////////////////////
// API CALLS

// ListModels returns all the models
func (c *Client) ListModels(ctx context.Context) ([]Model, error) {
	// Return the response
	var response struct {
		Data []Model `json:"data"`
	}
	if err := c.DoWithContext(ctx, nil, &response, client.OptPath("models")); err != nil {
		return nil, err
	}

	// Return success
	return response.Data, nil
}

// GetModel returns one model
func (c *Client) GetModel(ctx context.Context, model string) (*Model, error) {
	// Return the response
	var response Model
	if err := c.DoWithContext(ctx, nil, &response, client.OptPath("models", model)); err != nil {
		return nil, err
	}

	// Return success
	return &response, nil
}

// Delete a fine-tuned model. You must have the Owner role in your organization
// to delete a model.
func (c *Client) DeleteModel(ctx context.Context, model string) error {
	if err := c.DoWithContext(ctx, client.MethodDelete, nil, client.OptPath("models", model)); err != nil {
		return err
	}

	// Return success
	return nil
}
