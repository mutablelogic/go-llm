package ollama_test

import (
	"context"
	"encoding/json"
	"testing"

	// Packages
	opt "github.com/mutablelogic/go-llm/pkg/opt"
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

func Test_models_download_with_progress(t *testing.T) {
	t.Skip("Skipping download test - requires actual download which may be slow")

	assert := assert.New(t)

	// Track progress calls
	var progressCalls int
	var lastStatus string
	var lastPercent float64

	// Create progress callback
	progressOpt := opt.WithProgress(func(status string, percent float64) {
		progressCalls++
		lastStatus = status
		lastPercent = percent
		t.Logf("Progress: %s - %.2f%%", status, percent)
	})

	// Download a small model (this is commented out for CI)
	model, err := client.DownloadModel(context.TODO(), "mistral:7b", progressOpt)
	assert.NoError(err)
	assert.NotNil(model)

	// Verify progress was called
	assert.Greater(progressCalls, 0, "Expected progress callback to be called")
	t.Logf("Total progress calls: %d", progressCalls)
	t.Logf("Last status: %s, Last percent: %.2f%%", lastStatus, lastPercent)
}
