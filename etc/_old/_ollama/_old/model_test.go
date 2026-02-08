package ollama_test

import (
	"context"
	"testing"

	// Packages

	ollama "github.com/mutablelogic/go-llm/pkg/ollama"
	assert "github.com/stretchr/testify/assert"
)

func Test_model_001(t *testing.T) {
	var names []string

	t.Run("Models", func(t *testing.T) {
		// Get all models
		assert := assert.New(t)
		models, err := client.Models(context.TODO())
		if !assert.NoError(err) {
			t.FailNow()
		}
		assert.NotNil(models)
		for _, model := range models {
			names = append(names, model.Name())
		}
	})
	t.Run("Model", func(t *testing.T) {
		// Get models one by one
		assert := assert.New(t)
		for _, name := range names {
			model, err := client.GetModel(context.TODO(), name)
			if !assert.NoError(err) {
				t.FailNow()
			}
			assert.NotNil(model)
		}
	})
	t.Run("PullModel", func(t *testing.T) {
		// Pull a model
		assert := assert.New(t)
		model, err := client.PullModel(context.TODO(), "qwen:0.5b", ollama.WithPullStatus(func(status *ollama.PullStatus) {
			t.Log(status)
		}))
		if !assert.NoError(err) {
			t.FailNow()
		}
		assert.NotNil(model)
	})

	t.Run("CopyModel", func(t2 *testing.T) {
		// Copy a model
		assert := assert.New(t)
		err := client.CopyModel(context.TODO(), "qwen:0.5b", t.Name())
		if !assert.NoError(err) {
			t.FailNow()
		}
	})

	t.Run("LoadModel", func(t2 *testing.T) {
		// Load model into memory
		assert := assert.New(t)
		_, err := client.LoadModel(context.TODO(), t.Name())
		assert.NoError(err)
	})

	t.Run("UnloadModel", func(t2 *testing.T) {
		// Unload model from memory
		assert := assert.New(t)
		err := client.UnloadModel(context.TODO(), t.Name())
		assert.NoError(err)
	})

	t.Run("DeleteModel", func(t2 *testing.T) {
		// Delete a model
		assert := assert.New(t)
		_, err := client.GetModel(context.TODO(), t.Name())
		if assert.NoError(err) {
			err = client.DeleteModel(context.TODO(), t.Name())
			assert.NoError(err)
		}
	})

}
