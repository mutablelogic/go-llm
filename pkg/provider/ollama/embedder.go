package ollama

import (
	"context"

	// Packages
	client "github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
	opt "github.com/mutablelogic/go-llm/pkg/opt"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
)

///////////////////////////////////////////////////////////////////////////////
// INTERFACE CHECK

var _ llm.Embedder = (*Client)(nil)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Embedding generates an embedding vector for a single text using the specified model.
func (c *Client) Embedding(ctx context.Context, model schema.Model, text string, opts ...opt.Opt) ([]float64, error) {
	vectors, err := c.BatchEmbedding(ctx, model, []string{text}, opts...)
	if err != nil {
		return nil, err
	}
	if len(vectors) == 0 {
		return nil, schema.ErrNotFound.With("no embedding returned")
	}
	return vectors[0], nil
}

// BatchEmbedding generates embedding vectors for multiple texts using the specified model.
func (c *Client) BatchEmbedding(ctx context.Context, model schema.Model, texts []string, _ ...opt.Opt) ([][]float64, error) {
	if len(texts) == 0 {
		return nil, schema.ErrBadParameter.With("at least one text is required")
	}

	payload, err := client.NewJSONRequest(embedRequest{
		Model: model.Name,
		Input: texts,
	})
	if err != nil {
		return nil, err
	}

	var resp embedResponse
	if err := c.DoWithContext(ctx, payload, &resp, client.OptPath("embed")); err != nil {
		return nil, err
	}

	return resp.Embeddings, nil
}
