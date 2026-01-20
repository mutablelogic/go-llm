package ollama

import (
	"context"
	"encoding/json"
	"time"

	// Packages
	client "github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// model is the implementation of the llm.Embedding interface
type embedding struct {
	EmbeddingMeta
}

// EmbeddingMeta is the metadata for a generated embedding vector
type EmbeddingMeta struct {
	Model      string      `json:"model"`
	Embeddings [][]float64 `json:"embeddings"`
	Metrics
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
// PUBLIC METHODS

type reqEmbedding struct {
	Model     string                 `json:"model"`
	Input     []string               `json:"input"`
	KeepAlive *time.Duration         `json:"keep_alive,omitempty"`
	Truncate  *bool                  `json:"truncate,omitempty"`
	Options   map[string]interface{} `json:"options,omitempty"`
}

func (ollama *Client) GenerateEmbedding(ctx context.Context, name string, prompt []string, opts ...llm.Opt) (*EmbeddingMeta, error) {
	// Apply options
	opt, err := llm.ApplyOpts(opts...)
	if err != nil {
		return nil, err
	}

	// Bail out is no prompt
	if len(prompt) == 0 {
		return nil, llm.ErrBadParameter.With("missing prompt")
	}

	// Request
	req, err := client.NewJSONRequest(reqEmbedding{
		Model:     name,
		Input:     prompt,
		Truncate:  optTruncate(opt),
		KeepAlive: optKeepAlive(opt),
		Options:   optOptions(opt),
	})
	if err != nil {
		return nil, err
	}

	//  Response
	var response embedding
	if err := ollama.DoWithContext(ctx, req, &response, client.OptPath("embed")); err != nil {
		return nil, err
	}

	// Return success
	return &response.EmbeddingMeta, nil
}

// Embedding vector generation
func (model *model) Embedding(ctx context.Context, prompt string, opts ...llm.Opt) ([]float64, error) {
	embedding, err := model.GenerateEmbedding(ctx, model.Name(), []string{prompt}, opts...)
	if err != nil {
		return nil, err
	}
	if len(embedding.Embeddings) > 0 {
		return embedding.Embeddings[0], nil
	}
	return nil, llm.ErrNotFound.With("no embeddings returned")
}
