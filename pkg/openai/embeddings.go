package openai

import (
	"context"
	"encoding/json"

	// Packages
	client "github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// embeddings is the implementation of the llm.Embedding interface
type embeddings struct {
	Embeddings
}

// Embeddings is the metadata for a generated embedding vector
type Embeddings struct {
	Type  string      `json:"object"`
	Model string      `json:"model"`
	Data  []Embedding `json:"data"`
	Metrics
}

// Embedding is a single vector
type Embedding struct {
	Type   string    `json:"object"`
	Index  uint64    `json:"index"`
	Vector []float64 `json:"embedding"`
}

///////////////////////////////////////////////////////////////////////////////
// STRINGIFY

func (m Embedding) MarshalJSON() ([]byte, error) {
	return json.Marshal(m.Vector)
}

func (m embeddings) MarshalJSON() ([]byte, error) {
	return json.Marshal(m.Embeddings)
}

func (m embeddings) String() string {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err.Error()
	}
	return string(data)
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

type reqEmbedding struct {
	Model      string   `json:"model"`
	Input      []string `json:"input"`
	Format     string   `json:"encoding_format,omitempty"`
	Dimensions uint64   `json:"dimensions,omitempty"`
	User       string   `json:"user,omitempty"`
}

func (openai *Client) GenerateEmbedding(ctx context.Context, model string, prompt []string, opts ...llm.Opt) (*embeddings, error) {
	// Bail out is no prompt
	if len(prompt) == 0 {
		return nil, llm.ErrBadParameter.With("missing prompt")
	}

	// Apply options
	opt, err := llm.ApplyOpts(opts...)
	if err != nil {
		return nil, err
	}

	// Request
	req, err := client.NewJSONRequest(reqEmbedding{
		Model:      model,
		Input:      prompt,
		Format:     optFormat(opt),
		Dimensions: optDimensions(opt),
		User:       optUser(opt),
	})
	if err != nil {
		return nil, err
	}

	//  Response
	var response embeddings
	if err := openai.DoWithContext(ctx, req, &response, client.OptPath("embeddings")); err != nil {
		return nil, err
	}

	// Return success
	return &response, nil
}

// Generate one vector
func (model *model) Embedding(ctx context.Context, prompt string, opts ...llm.Opt) ([]float64, error) {
	response, err := model.GenerateEmbedding(ctx, model.Name(), []string{prompt}, opts...)
	if err != nil {
		return nil, err
	}
	if len(response.Embeddings.Data) == 0 {
		return nil, llm.ErrNotFound.With("no embeddings returned")
	}
	return response.Embeddings.Data[0].Vector, nil
}
