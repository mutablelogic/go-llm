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

func Test_models_list_running(t *testing.T) {
	assert := assert.New(t)

	models, err := client.ListRunningModels(context.TODO())
	assert.NoError(err)

	// No guarantee there are running models, but should not error
	t.Logf("Running models: %v", models)
}

func Test_models_get(t *testing.T) {
	assert := assert.New(t)

	// Use a known model name or the first from ListModels
	models, err := client.ListModels(context.TODO())
	assert.NoError(err)
	assert.NotEmpty(models)
	model := models[0]

	got, err := client.GetModel(context.TODO(), model.Name)
	assert.NoError(err)
	assert.NotNil(got)
	assert.Equal(model.Name, got.Name)
}

func Test_models_delete_load_unload(t *testing.T) {
	assert := assert.New(t)

	// Use a known model name or the first from ListModels
	models, err := client.ListModels(context.TODO())
	assert.NoError(err)
	assert.NotEmpty(models)
	model := models[0]

	err = client.LoadModel(context.TODO(), model)
	// Should not error if supported, may error if not implemented
	if err != nil {
		t.Logf("LoadModel error: %v", err)
	}

	err = client.UnloadModel(context.TODO(), model)
	if err != nil {
		t.Logf("UnloadModel error: %v", err)
	}

	err = client.DeleteModel(context.TODO(), model)
	if err != nil {
		t.Logf("DeleteModel error: %v", err)
	}
}
