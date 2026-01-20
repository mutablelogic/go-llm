package openai_test

import (
	"context"
	"testing"

	// Packages
	assert "github.com/stretchr/testify/assert"
)

func Test_models_001(t *testing.T) {
	assert := assert.New(t)

	response, err := client.ListModels(context.TODO())
	assert.NoError(err)
	assert.NotEmpty(response)

	t.Run("models", func(t *testing.T) {
		for _, model := range response {
			model_, err := client.GetModel(context.TODO(), model.Name)
			if assert.NoError(err) {
				assert.NotNil(model_)
				assert.Equal(*model_, model)
			}
		}
	})
}

func Test_models_002(t *testing.T) {
	assert := assert.New(t)

	response, err := client.Models(context.TODO())
	assert.NoError(err)
	assert.NotEmpty(response)

	t.Run("models", func(t *testing.T) {
		for _, model := range response {
			model_ := client.Model(context.TODO(), model.Name())
			assert.NotNil(model_)
			assert.Equal(model_, model)
		}
	})
}
