package agent

import (
	"context"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	opt "github.com/mutablelogic/go-llm/pkg/opt"
	google "github.com/mutablelogic/go-llm/pkg/provider/google"
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

	// Convert options based on client
	opts, err := convertOptsForClient(opts, client)
	if err != nil {
		return nil, err
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

	// Convert options based on client
	opts, err := convertOptsForClient(opts, client)
	if err != nil {
		return nil, err
	}

	embedder, ok := client.(llm.Embedder)
	if !ok {
		return nil, llm.ErrNotImplemented.With("client does not support embeddings")
	}

	return embedder.BatchEmbedding(ctx, model, texts, opts...)
}

///////////////////////////////////////////////////////////////////////////////
// AGENT-LEVEL EMBEDDING OPTIONS

// WithTaskType sets the task type for the embedding request, dispatching to the
// correct provider-specific option at call time.
func WithTaskType(taskType string) opt.Opt {
	return opt.WithClient(func(provider string) opt.Opt {
		switch provider {
		case schema.Gemini:
			return google.WithTaskType(taskType)
		default:
			return opt.Error(llm.ErrNotImplemented.Withf("%s: WithTaskType not supported", provider))
		}
	})
}

// WithTitle sets the title for the embedding request, dispatching to the
// correct provider-specific option at call time.
func WithTitle(title string) opt.Opt {
	return opt.WithClient(func(provider string) opt.Opt {
		switch provider {
		case schema.Gemini:
			return google.WithTitle(title)
		default:
			return opt.Error(llm.ErrNotImplemented.Withf("%s: WithTitle not supported", provider))
		}
	})
}

// WithOutputDimensionality sets the output dimensionality for the embedding,
// dispatching to the correct provider-specific option at call time.
func WithOutputDimensionality(d uint) opt.Opt {
	return opt.WithClient(func(provider string) opt.Opt {
		switch provider {
		case schema.Gemini:
			return google.WithOutputDimensionality(d)
		default:
			return opt.Error(llm.ErrNotImplemented.Withf("%s: WithOutputDimensionality not supported", provider))
		}
	})
}
