package llm

import (
	"context"

	// Packages
	schema "github.com/mutablelogic/go-llm/pkg/schema"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type Client interface {
	// Return the provider name
	Name() string

	// ListModels returns the list of available models
	ListModels(ctx context.Context) ([]schema.Model, error)

	// GetModel returns the model with the given name
	GetModel(ctx context.Context, name string) (*schema.Model, error)
}
