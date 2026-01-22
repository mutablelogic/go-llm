package google_test

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

func Test_models_002(t *testing.T) {
	assert := assert.New(t)

	// Get a specific model
	model, err := client.GetModel(context.TODO(), "gemini-2.0-flash")
	assert.NoError(err)
	assert.NotNil(model)
	assert.Equal("gemini-2.0-flash", model.Name)

	data, err := json.MarshalIndent(model, "", "  ")
	assert.NoError(err)
	t.Log(string(data))
}
