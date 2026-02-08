package agent

import (
	"context"
	"sort"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	opt "github.com/mutablelogic/go-llm/pkg/opt"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// ListModels returns the list of available models from all clients
func (a *agent) ListModels(ctx context.Context, opts ...opt.Opt) ([]schema.Model, error) {
	var result []schema.Model

	// Apply options
	o, err := opt.Apply(opts...)
	if err != nil {
		return nil, err
	}

	for _, client := range a.clients {
		// Match the provider option
		if !matchProvider(o, client.Name()) {
			continue
		}

		// List models for this provider
		models, err := client.ListModels(ctx)
		if err != nil {
			return nil, err
		}

		// Append the models
		result = append(result, models...)
	}

	// Sort the models by name
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })

	// Return the models
	return result, nil
}

// GetModel returns the model with the given name from any client
func (a *agent) GetModel(ctx context.Context, name string, opts ...opt.Opt) (*schema.Model, error) {
	// Apply options
	o, err := opt.Apply(opts...)
	if err != nil {
		return nil, err
	}

	// Get model based on name
	for _, client := range a.clients {
		// Match the provider option
		if !matchProvider(o, client.Name()) {
			continue
		}

		// Match the model
		model, err := client.GetModel(ctx, name)
		if err == nil && model != nil {
			return model, nil
		}
	}

	// Return "not found" error
	return nil, llm.ErrNotFound
}
