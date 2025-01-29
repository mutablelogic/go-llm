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
	opt, err := apply(opts...)
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
		Truncate:  opt.truncate,
		KeepAlive: opt.keepalive,
		Options:   opt.options,
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
func (ollama *Client) Embedding(context.Context, llm.Model, string, ...llm.Opt) ([]float64, error) {
	return nil, llm.ErrNotImplemented
}
