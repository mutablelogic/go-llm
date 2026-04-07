package ollama_test

import (
	"context"
	"strings"
	"testing"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	ollama "github.com/mutablelogic/go-llm/pkg/provider/ollama"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	assert "github.com/stretchr/testify/assert"
)

///////////////////////////////////////////////////////////////////////////////
// HELPERS

// skipIfEmbeddingUnsupported skips the test when the model reports it does not
// support the embed endpoint (Ollama 501 response).
func skipIfEmbeddingUnsupported(t *testing.T, err error) {
	t.Helper()
	if err != nil && strings.Contains(err.Error(), "does not support embeddings") {
		t.Skipf("model does not support embeddings: %v", err)
	}
}

///////////////////////////////////////////////////////////////////////////////
// UNIT TESTS

func Test_embedding_001(t *testing.T) {
	// Test that Client satisfies the llm.Embedder interface
	a := assert.New(t)
	c, err := ollama.New("")
	a.NoError(err)
	var _ llm.Embedder = c
	a.NotNil(c)
}

func Test_embedding_002(t *testing.T) {
	// Test that BatchEmbedding with empty input returns an error
	a := assert.New(t)
	c, err := ollama.New("")
	a.NoError(err)

	model := schema.Model{Name: "test-model"}
	_, _, err = c.BatchEmbedding(context.TODO(), model, []string{})
	a.Error(err)
}

///////////////////////////////////////////////////////////////////////////////
// INTEGRATION TESTS

func Test_embedding_003(t *testing.T) {
	// Test single embedding against the running Ollama instance
	c := requireClient(t)
	a := assert.New(t)

	name := firstModel(t, c)
	model, err := c.GetModel(context.Background(), name)
	skipIfUnreachable(t, err)
	if !a.NoError(err) || !a.NotNil(model) {
		t.FailNow()
	}

	vec, usage, err := c.Embedding(context.Background(), *model, "Hello, world!")
	skipIfUnreachable(t, err)
	skipIfEmbeddingUnsupported(t, err)
	a.NoError(err)
	a.NotEmpty(vec)
	a.Nil(usage)
	t.Logf("Got embedding vector with %d dimensions", len(vec))
}

func Test_embedding_004(t *testing.T) {
	// Test batch embeddings against the running Ollama instance
	c := requireClient(t)
	a := assert.New(t)

	name := firstModel(t, c)
	model, err := c.GetModel(context.Background(), name)
	skipIfUnreachable(t, err)
	if !a.NoError(err) || !a.NotNil(model) {
		t.FailNow()
	}

	vecs, usage, err := c.BatchEmbedding(context.Background(), *model, []string{"Hello", "World"})
	skipIfUnreachable(t, err)
	skipIfEmbeddingUnsupported(t, err)
	a.NoError(err)
	a.Nil(usage)
	if a.Len(vecs, 2) {
		for _, v := range vecs {
			a.NotEmpty(v)
		}
		t.Logf("Got %d embedding vectors, first has %d dimensions", len(vecs), len(vecs[0]))
	}
}
