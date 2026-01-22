package ollama_test

import (
	"context"
	"encoding/json"
	"testing"

	// Packages
	assert "github.com/stretchr/testify/assert"
)

func Test_models_001(t *testing.T) {
	assert := assert.New(t)

	models, err := client.ListModels(context.TODO())
	assert.NoError(err)
	assert.NotEmpty(models)

	data, err := json.MarshalIndent(models, "", "  ")
	assert.NoError(err)
	t.Log(string(data))
}
