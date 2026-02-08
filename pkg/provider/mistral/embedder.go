package mistral

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
		return nil, llm.ErrNotFound.With("no embedding returned")
	}
	return vectors[0], nil
}

// BatchEmbedding generates embedding vectors for multiple texts using the specified model.
func (c *Client) BatchEmbedding(ctx context.Context, model schema.Model, texts []string, _ ...opt.Opt) ([][]float64, error) {
	if len(texts) == 0 {
		return nil, llm.ErrBadParameter.With("at least one text is required")
	}

	req := embeddingsRequest{
		Model: model.Name,
		Input: texts,
	}

	payload, err := client.NewJSONRequest(req)
	if err != nil {
		return nil, err
	}

	var resp embeddingsResponse
	if err := c.DoWithContext(ctx, payload, &resp, client.OptPath("embeddings")); err != nil {
		return nil, err
	}

	result := make([][]float64, 0, len(resp.Data))
	for _, item := range resp.Data {
		result = append(result, item.Embedding)
	}
	return result, nil
}
