package ollama_test

import (
	"context"
	"os"
	"testing"

	// Packages
	opts "github.com/mutablelogic/go-client"
	ollama "github.com/mutablelogic/go-llm/pkg/ollama"
	assert "github.com/stretchr/testify/assert"
)

func Test_embed_001(t *testing.T) {
	client, err := ollama.New(GetEndpoint(t), opts.OptTrace(os.Stderr, true))
	if err != nil {
		t.FailNow()
	}

	t.Run("Embedding", func(t *testing.T) {
		assert := assert.New(t)
		embedding, err := client.GenerateEmbedding(context.TODO(), "qwen:0.5b", []string{"world"})
		if !assert.NoError(err) {
			t.FailNow()
		}
		t.Log(embedding)
	})
}
