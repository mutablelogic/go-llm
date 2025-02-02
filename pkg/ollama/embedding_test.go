package ollama_test

import (
	"context"
	"testing"

	// Packages

	assert "github.com/stretchr/testify/assert"
)

func Test_embed_001(t *testing.T) {
	t.Run("Embedding", func(t *testing.T) {
		assert := assert.New(t)
		embedding, err := client.GenerateEmbedding(context.TODO(), "qwen:0.5b", []string{"hello, world"})
		if !assert.NoError(err) {
			t.FailNow()
		}
		t.Log(embedding)
	})
}
