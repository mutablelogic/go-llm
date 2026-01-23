package agent

import (
	"context"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	opt "github.com/mutablelogic/go-llm/pkg/opt"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
)

///////////////////////////////////////////////////////////////////////////////
// INTERFACE CHECK

var _ llm.Embedder = (*agent)(nil)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Embedding generates an embedding vector for a single text using the specified model
func (a *agent) Embedding(ctx context.Context, model schema.Model, text string, opts ...opt.Opt) ([]float64, error) {
	client := a.clientForModel(model)
	if client == nil {
		return nil, llm.ErrNotFound.With("no client found for model")
	}

	embedder, ok := client.(llm.Embedder)
	if !ok {
		return nil, llm.ErrNotImplemented.With("client does not support embeddings")
	}

	return embedder.Embedding(ctx, model, text, opts...)
}

// BatchEmbedding generates embedding vectors for multiple texts using the specified model
func (a *agent) BatchEmbedding(ctx context.Context, model schema.Model, texts []string, opts ...opt.Opt) ([][]float64, error) {
	client := a.clientForModel(model)
	if client == nil {
		return nil, llm.ErrNotFound.With("no client found for model")
	}

	embedder, ok := client.(llm.Embedder)
	if !ok {
		return nil, llm.ErrNotImplemented.With("client does not support embeddings")
	}

	return embedder.BatchEmbedding(ctx, model, texts, opts...)
}
