package agent

import (
	"context"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// ListModels returns the list of available models from all clients
func (a *agent) ListModels(ctx context.Context) ([]schema.Model, error) {
	var result []schema.Model
	for _, client := range a.clients {
		models, err := client.ListModels(ctx)
		if err != nil {
			return nil, err
		}
		result = append(result, models...)
	}
	return result, nil
}

// GetModel returns the model with the given name from any client
func (a *agent) GetModel(ctx context.Context, name string) (*schema.Model, error) {
	for _, client := range a.clients {
		model, err := client.GetModel(ctx, name)
		if err == nil && model != nil {
			return model, nil
		}
	}
	return nil, llm.ErrNotFound
}
