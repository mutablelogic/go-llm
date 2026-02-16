package manager

import (
	"context"
	"testing"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	opt "github.com/mutablelogic/go-llm/pkg/opt"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	assert "github.com/stretchr/testify/assert"
)

///////////////////////////////////////////////////////////////////////////////
// MOCK EMBEDDER CLIENT

// mockEmbedderClient implements llm.Client + llm.Embedder
type mockEmbedderClient struct {
	mockClient
	embeddingFn      func(ctx context.Context, model schema.Model, text string, opts ...opt.Opt) ([]float64, error)
	batchEmbeddingFn func(ctx context.Context, model schema.Model, texts []string, opts ...opt.Opt) ([][]float64, error)
}

func (c *mockEmbedderClient) Embedding(ctx context.Context, model schema.Model, text string, opts ...opt.Opt) ([]float64, error) {
	if c.embeddingFn != nil {
		return c.embeddingFn(ctx, model, text, opts...)
	}
	return []float64{0.1, 0.2, 0.3}, nil
}

func (c *mockEmbedderClient) BatchEmbedding(ctx context.Context, model schema.Model, texts []string, opts ...opt.Opt) ([][]float64, error) {
	if c.batchEmbeddingFn != nil {
		return c.batchEmbeddingFn(ctx, model, texts, opts...)
	}
	result := make([][]float64, len(texts))
	for i := range texts {
		result[i] = []float64{0.1, 0.2, 0.3}
	}
	return result, nil
}

///////////////////////////////////////////////////////////////////////////////
// EMBEDDING TESTS

// Test successful single-input embedding
func Test_embedding_001(t *testing.T) {
	assert := assert.New(t)

	client := &mockEmbedderClient{
		mockClient: mockClient{
			name:   "embed-provider",
			models: []schema.Model{{Name: "embed-model", OwnedBy: "embed-provider"}},
		},
		embeddingFn: func(_ context.Context, _ schema.Model, text string, _ ...opt.Opt) ([]float64, error) {
			return []float64{1.0, 2.0, 3.0, 4.0}, nil
		},
	}

	m, err := NewManager(WithClient(client))
	assert.NoError(err)

	resp, err := m.Embedding(context.TODO(), &schema.EmbeddingRequest{
		Provider: "embed-provider",
		Model:    "embed-model",
		Input:    []string{"hello world"},
	})
	assert.NoError(err)
	assert.NotNil(resp)
	assert.Equal("embed-provider", resp.Provider)
	assert.Equal("embed-model", resp.Model)
	assert.Len(resp.Output, 1)
	assert.Equal([]float64{1.0, 2.0, 3.0, 4.0}, resp.Output[0])
	assert.Equal(uint(4), resp.OutputDimensionality)
}

// Test successful multi-input (batch) embedding
func Test_embedding_002(t *testing.T) {
	assert := assert.New(t)

	client := &mockEmbedderClient{
		mockClient: mockClient{
			name:   "embed-provider",
			models: []schema.Model{{Name: "embed-model", OwnedBy: "embed-provider"}},
		},
		batchEmbeddingFn: func(_ context.Context, _ schema.Model, texts []string, _ ...opt.Opt) ([][]float64, error) {
			result := make([][]float64, len(texts))
			for i := range texts {
				result[i] = []float64{float64(i), float64(i + 1)}
			}
			return result, nil
		},
	}

	m, err := NewManager(WithClient(client))
	assert.NoError(err)

	resp, err := m.Embedding(context.TODO(), &schema.EmbeddingRequest{
		Provider: "embed-provider",
		Model:    "embed-model",
		Input:    []string{"first", "second", "third"},
	})
	assert.NoError(err)
	assert.NotNil(resp)
	assert.Len(resp.Output, 3)
	assert.Equal([]float64{0.0, 1.0}, resp.Output[0])
	assert.Equal([]float64{1.0, 2.0}, resp.Output[1])
	assert.Equal([]float64{2.0, 3.0}, resp.Output[2])
	assert.Equal(uint(2), resp.OutputDimensionality)
}

// Test empty input returns error
func Test_embedding_003(t *testing.T) {
	assert := assert.New(t)

	client := &mockEmbedderClient{
		mockClient: mockClient{
			name:   "embed-provider",
			models: []schema.Model{{Name: "embed-model", OwnedBy: "embed-provider"}},
		},
	}

	m, err := NewManager(WithClient(client))
	assert.NoError(err)

	_, err = m.Embedding(context.TODO(), &schema.EmbeddingRequest{
		Provider: "embed-provider",
		Model:    "embed-model",
		Input:    []string{},
	})
	assert.ErrorIs(err, llm.ErrBadParameter)
}

// Test model not found returns error
func Test_embedding_004(t *testing.T) {
	assert := assert.New(t)

	client := &mockEmbedderClient{
		mockClient: mockClient{
			name:   "embed-provider",
			models: []schema.Model{{Name: "embed-model", OwnedBy: "embed-provider"}},
		},
	}

	m, err := NewManager(WithClient(client))
	assert.NoError(err)

	_, err = m.Embedding(context.TODO(), &schema.EmbeddingRequest{
		Provider: "embed-provider",
		Model:    "nonexistent",
		Input:    []string{"hello"},
	})
	assert.ErrorIs(err, llm.ErrNotFound)
}

// Test client that does not implement Embedder returns ErrNotImplemented
func Test_embedding_005(t *testing.T) {
	assert := assert.New(t)

	// plain mockClient does not implement llm.Embedder
	client := &mockClient{
		name:   "plain-provider",
		models: []schema.Model{{Name: "plain-model", OwnedBy: "plain-provider"}},
	}

	m, err := NewManager(WithClient(client))
	assert.NoError(err)

	_, err = m.Embedding(context.TODO(), &schema.EmbeddingRequest{
		Provider: "plain-provider",
		Model:    "plain-model",
		Input:    []string{"hello"},
	})
	assert.ErrorIs(err, llm.ErrNotImplemented)
}

// Test default task type is applied when empty
func Test_embedding_006(t *testing.T) {
	assert := assert.New(t)

	client := &mockEmbedderClient{
		mockClient: mockClient{
			name:   "embed-provider",
			models: []schema.Model{{Name: "embed-model", OwnedBy: "embed-provider"}},
		},
	}

	m, err := NewManager(WithClient(client))
	assert.NoError(err)

	resp, err := m.Embedding(context.TODO(), &schema.EmbeddingRequest{
		Provider: "embed-provider",
		Model:    "embed-model",
		Input:    []string{"hello"},
		// TaskType left empty
	})
	assert.NoError(err)
	assert.Equal(schema.EmbeddingTaskTypeDefault, resp.TaskType)
}

// Test custom task type is preserved in response
func Test_embedding_007(t *testing.T) {
	assert := assert.New(t)

	client := &mockEmbedderClient{
		mockClient: mockClient{
			name:   "embed-provider",
			models: []schema.Model{{Name: "embed-model", OwnedBy: "embed-provider"}},
		},
	}

	m, err := NewManager(WithClient(client))
	assert.NoError(err)

	resp, err := m.Embedding(context.TODO(), &schema.EmbeddingRequest{
		Provider: "embed-provider",
		Model:    "embed-model",
		Input:    []string{"hello"},
		TaskType: "RETRIEVAL_QUERY",
	})
	assert.NoError(err)
	assert.Equal("RETRIEVAL_QUERY", resp.TaskType)
}

// Test title is preserved in response
func Test_embedding_008(t *testing.T) {
	assert := assert.New(t)

	client := &mockEmbedderClient{
		mockClient: mockClient{
			name:   "embed-provider",
			models: []schema.Model{{Name: "embed-model", OwnedBy: "embed-provider"}},
		},
	}

	m, err := NewManager(WithClient(client))
	assert.NoError(err)

	resp, err := m.Embedding(context.TODO(), &schema.EmbeddingRequest{
		Provider: "embed-provider",
		Model:    "embed-model",
		Input:    []string{"hello"},
		TaskType: "RETRIEVAL_DOCUMENT",
		Title:    "My Document",
	})
	assert.NoError(err)
	assert.Equal("My Document", resp.Title)
}

// Test provider not found (no clients registered)
func Test_embedding_009(t *testing.T) {
	assert := assert.New(t)

	m, err := NewManager()
	assert.NoError(err)

	_, err = m.Embedding(context.TODO(), &schema.EmbeddingRequest{
		Provider: "nonexistent",
		Model:    "some-model",
		Input:    []string{"hello"},
	})
	assert.ErrorIs(err, llm.ErrNotFound)
}

// Test model found without provider filter (searches all clients)
func Test_embedding_010(t *testing.T) {
	assert := assert.New(t)

	client := &mockEmbedderClient{
		mockClient: mockClient{
			name:   "embed-provider",
			models: []schema.Model{{Name: "embed-model", OwnedBy: "embed-provider"}},
		},
	}

	m, err := NewManager(WithClient(client))
	assert.NoError(err)

	resp, err := m.Embedding(context.TODO(), &schema.EmbeddingRequest{
		Model: "embed-model", // No provider specified
		Input: []string{"hello"},
	})
	assert.NoError(err)
	assert.NotNil(resp)
	assert.Equal("embed-provider", resp.Provider)
	assert.Equal("embed-model", resp.Model)
}

// Test embedding error is propagated
func Test_embedding_011(t *testing.T) {
	assert := assert.New(t)

	client := &mockEmbedderClient{
		mockClient: mockClient{
			name:   "embed-provider",
			models: []schema.Model{{Name: "embed-model", OwnedBy: "embed-provider"}},
		},
		embeddingFn: func(_ context.Context, _ schema.Model, _ string, _ ...opt.Opt) ([]float64, error) {
			return nil, llm.ErrBadParameter.With("upstream error")
		},
	}

	m, err := NewManager(WithClient(client))
	assert.NoError(err)

	_, err = m.Embedding(context.TODO(), &schema.EmbeddingRequest{
		Provider: "embed-provider",
		Model:    "embed-model",
		Input:    []string{"hello"},
	})
	assert.Error(err)
	assert.ErrorIs(err, llm.ErrBadParameter)
}

// Test batch embedding error is propagated
func Test_embedding_012(t *testing.T) {
	assert := assert.New(t)

	client := &mockEmbedderClient{
		mockClient: mockClient{
			name:   "embed-provider",
			models: []schema.Model{{Name: "embed-model", OwnedBy: "embed-provider"}},
		},
		batchEmbeddingFn: func(_ context.Context, _ schema.Model, _ []string, _ ...opt.Opt) ([][]float64, error) {
			return nil, llm.ErrBadParameter.With("batch upstream error")
		},
	}

	m, err := NewManager(WithClient(client))
	assert.NoError(err)

	_, err = m.Embedding(context.TODO(), &schema.EmbeddingRequest{
		Provider: "embed-provider",
		Model:    "embed-model",
		Input:    []string{"hello", "world"},
	})
	assert.Error(err)
	assert.ErrorIs(err, llm.ErrBadParameter)
}
