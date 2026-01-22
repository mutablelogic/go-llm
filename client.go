package llm

import (
	"context"

	// Packages
	opt "github.com/mutablelogic/go-llm/pkg/opt"
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

// Embedder is an interface for generating text embeddings
type Embedder interface {
	// Embedding generates an embedding vector for a single text
	Embedding(ctx context.Context, model string, text string, opts ...opt.Opt) ([]float64, error)

	// BatchEmbedding generates embedding vectors for multiple texts
	BatchEmbedding(ctx context.Context, model string, texts []string, opts ...opt.Opt) ([][]float64, error)
}
