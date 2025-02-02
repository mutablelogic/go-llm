package ollama_test

import (
	"context"
	"testing"

	// Packages

	assert "github.com/stretchr/testify/assert"
)

func Test_embeddings_001(t *testing.T) {
	t.Run("Embedding1", func(t *testing.T) {
		assert := assert.New(t)
		embedding, err := client.GenerateEmbedding(context.TODO(), "qwen:0.5b", []string{"hello, world"})
		if !assert.NoError(err) {
			t.FailNow()
		}
		assert.Equal(1, len(embedding.Embeddings))
	})

	t.Run("Embedding2", func(t *testing.T) {
		assert := assert.New(t)
		embedding, err := client.GenerateEmbedding(context.TODO(), "qwen:0.5b", []string{"hello, world", "goodbye cruel world"})
		if !assert.NoError(err) {
			t.FailNow()
		}
		assert.Equal(2, len(embedding.Embeddings))
	})
}
