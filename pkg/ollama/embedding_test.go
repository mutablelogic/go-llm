package ollama_test

import (
	"context"
	"testing"

	// Packages
	assert "github.com/stretchr/testify/assert"

	schema "github.com/mutablelogic/go-llm/pkg/schema"
)

func Test_embeddings_001(t *testing.T) {
	model := schema.Model{Name: "qwen:0.5b"}

	t.Run("Embedding", func(t *testing.T) {
		assert := assert.New(t)
		vector, err := client.Embedding(context.TODO(), model, "hello, world")
		if !assert.NoError(err) {
			t.FailNow()
		}
		assert.NotEmpty(vector)
	})

	t.Run("BatchEmbedding", func(t *testing.T) {
		assert := assert.New(t)
		vectors, err := client.BatchEmbedding(context.TODO(), model, []string{"hello, world", "goodbye cruel world"})
		if !assert.NoError(err) {
			t.FailNow()
		}
		assert.Equal(2, len(vectors))
	})
}
