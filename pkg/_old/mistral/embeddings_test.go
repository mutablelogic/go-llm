package mistral_test

import (
	"context"
	"testing"

	// Packages
	assert "github.com/stretchr/testify/assert"
)

func Test_embeddings_001(t *testing.T) {
	assert := assert.New(t)
	model := client.Model(context.TODO(), "mistral-embed")
	if assert.NotNil(model) {
		response, err := model.Embedding(context.TODO(), "Hello, how are you?")
		assert.NoError(err)
		assert.NotEmpty(response)
		t.Log(response)
	}
}
