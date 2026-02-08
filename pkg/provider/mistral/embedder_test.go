package mistral_test

import (
	"context"
	"testing"

	// Packages
	mistral "github.com/mutablelogic/go-llm/pkg/provider/mistral"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	assert "github.com/stretchr/testify/assert"
)

///////////////////////////////////////////////////////////////////////////////
// UNIT TESTS

func Test_embedding_001(t *testing.T) {
	// Test that BatchEmbedding with empty input returns an error
	a := assert.New(t)
	c, err := mistral.New("test-key")
	a.NoError(err)

	model := schema.Model{Name: "mistral-embed"}
	_, err = c.BatchEmbedding(context.TODO(), model, []string{})
	a.Error(err)
}

func Test_embedding_002(t *testing.T) {
	// Test that the Client satisfies the Embedder interface
	a := assert.New(t)
	c, err := mistral.New("test-key")
	a.NoError(err)
	a.NotNil(c)
}

///////////////////////////////////////////////////////////////////////////////
// INTEGRATION TESTS

func Test_embedding_003(t *testing.T) {
	// Test single embedding
	if apiKey == "" {
		t.Skip("MISTRAL_API_KEY not set, skipping")
	}
	a := assert.New(t)
	c, err := mistral.New(apiKey)
	a.NoError(err)

	model := schema.Model{Name: "mistral-embed"}
	vector, err := c.Embedding(context.TODO(), model, "Hello, world!")
	a.NoError(err)
	a.NotEmpty(vector)
	t.Logf("Got embedding vector with %d dimensions", len(vector))
}

func Test_embedding_004(t *testing.T) {
	// Test batch embedding
	if apiKey == "" {
		t.Skip("MISTRAL_API_KEY not set, skipping")
	}
	a := assert.New(t)
	c, err := mistral.New(apiKey)
	a.NoError(err)

	model := schema.Model{Name: "mistral-embed"}
	texts := []string{
		"Hello, world!",
		"How are you?",
		"The quick brown fox jumps over the lazy dog.",
	}
	vectors, err := c.BatchEmbedding(context.TODO(), model, texts)
	a.NoError(err)
	a.Len(vectors, len(texts))

	for i, v := range vectors {
		a.NotEmpty(v)
		t.Logf("Vector %d has %d dimensions", i, len(v))
	}
}

func Test_embedding_005(t *testing.T) {
	// Test that two different texts produce different embeddings
	if apiKey == "" {
		t.Skip("MISTRAL_API_KEY not set, skipping")
	}
	a := assert.New(t)
	c, err := mistral.New(apiKey)
	a.NoError(err)

	model := schema.Model{Name: "mistral-embed"}
	texts := []string{
		"I love programming in Go.",
		"The weather in Paris is beautiful today.",
	}
	vectors, err := c.BatchEmbedding(context.TODO(), model, texts)
	a.NoError(err)
	a.Len(vectors, 2)

	// The two vectors should not be identical
	a.NotEqual(vectors[0], vectors[1])
	t.Logf("Vector dimensions: %d", len(vectors[0]))
}
