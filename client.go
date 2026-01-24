package llm

import (
	"context"

	// Packages
	opt "github.com/mutablelogic/go-llm/pkg/opt"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Client is the interface that wraps basic LLM client methods
type Client interface {
	// Return the provider name
	Name() string

	// ListModels returns the list of available models
	ListModels(context.Context, ...opt.Opt) ([]schema.Model, error)

	// GetModel returns the model with the given name
	GetModel(context.Context, string) (*schema.Model, error)
}

// Embedder is an interface for generating text embeddings
type Embedder interface {
	// Embedding generates an embedding vector for a single text
	Embedding(context.Context, schema.Model, string, ...opt.Opt) ([]float64, error)

	// BatchEmbedding generates embedding vectors for multiple texts
	BatchEmbedding(context.Context, schema.Model, []string, ...opt.Opt) ([][]float64, error)
}

// Downloader is an interface for managing model files
type Downloader interface {
	// DownloadModel downloads the specified model, and otherwise loads the model if already present
	DownloadModel(context.Context, string, ...opt.Opt) (*schema.Model, error)

	// DeleteModel deletes the specified model from local storage
	DeleteModel(context.Context, schema.Model) error
}

// Messenger is an interface for sending messages and conducting conversations
type Messenger interface {
	// WithoutSession sends a single message and returns the response (stateless)
	WithoutSession(context.Context, schema.Model, *schema.Message, ...opt.Opt) (*schema.Message, error)

	// WithSession sends a message within a session and returns the response (stateful)
	WithSession(context.Context, schema.Model, *schema.Session, *schema.Message, ...opt.Opt) (*schema.Message, error)
}
