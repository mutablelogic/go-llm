package ollama

import (
	"context"
	"encoding/json"

	// Packages
	client "github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
	opt "github.com/mutablelogic/go-llm/pkg/opt"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// embedding is the response structure for embed API
type embedding struct {
	EmbeddingMeta
}

// EmbeddingMeta is the metadata for a generated embedding vector
type EmbeddingMeta struct {
	Model      string      `json:"model"`
	Embeddings [][]float64 `json:"embeddings"`
}

///////////////////////////////////////////////////////////////////////////////
// STRINGIFY

func (m embedding) String() string {
	data, err := json.MarshalIndent(m.EmbeddingMeta, "", "  ")
	if err != nil {
		return err.Error()
	}
	return string(data)
}

func (m EmbeddingMeta) String() string {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err.Error()
	}
	return string(data)
}

///////////////////////////////////////////////////////////////////////////////
// INTERFACE CHECK

var _ llm.Embedder = (*Client)(nil)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

type reqEmbedding struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

// Embedding generates an embedding vector for a single text using the specified model
func (ollama *Client) Embedding(ctx context.Context, model schema.Model, text string, opts ...opt.Opt) ([]float64, error) {
	vectors, err := ollama.BatchEmbedding(ctx, model, []string{text}, opts...)
	if err != nil {
		return nil, err
	}
	if len(vectors) > 0 {
		return vectors[0], nil
	}
	return nil, llm.ErrNotFound.With("no embeddings returned")
}

// BatchEmbedding generates embedding vectors for multiple texts using the specified model
func (ollama *Client) BatchEmbedding(ctx context.Context, model schema.Model, texts []string, opts ...opt.Opt) ([][]float64, error) {
	// Bail out if no texts
	if len(texts) == 0 {
		return nil, llm.ErrBadParameter.With("at least one text is required")
	}

	// Request
	req, err := client.NewJSONRequest(reqEmbedding{
		Model: model.Name,
		Input: texts,
	})
	if err != nil {
		return nil, err
	}

	// Response
	var response embedding
	if err := ollama.DoWithContext(ctx, req, &response, client.OptPath("embed")); err != nil {
		return nil, err
	}

	// Return success
	return response.EmbeddingMeta.Embeddings, nil
}
