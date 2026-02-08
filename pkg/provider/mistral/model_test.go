package mistral_test

import (
	"context"
	"testing"

	// Packages
	mistral "github.com/mutablelogic/go-llm/pkg/provider/mistral"
	assert "github.com/stretchr/testify/assert"
)

///////////////////////////////////////////////////////////////////////////////
// TESTS

func Test_model_001(t *testing.T) {
	// Test that ListModels returns a non-empty list
	if apiKey == "" {
		t.Skip("MISTRAL_API_KEY not set, skipping")
	}
	assert := assert.New(t)
	client, err := mistral.New(apiKey)
	assert.NoError(err)

	models, err := client.ListModels(context.TODO())
	assert.NoError(err)
	assert.NotEmpty(models)

	// Every model should have a name and the correct owner
	for _, m := range models {
		assert.NotEmpty(m.Name)
		assert.Equal("mistral", m.OwnedBy)
		t.Logf("model: %s (%s)", m.Name, m.Description)
	}
}

func Test_model_002(t *testing.T) {
	// Test that GetModel returns a valid model for a known name
	if apiKey == "" {
		t.Skip("MISTRAL_API_KEY not set, skipping")
	}
	assert := assert.New(t)
	client, err := mistral.New(apiKey)
	assert.NoError(err)

	model, err := client.GetModel(context.TODO(), "mistral-small-latest")
	assert.NoError(err)
	assert.NotNil(model)
	assert.Equal("mistral-small-latest", model.Name)
	assert.Equal("mistral", model.OwnedBy)
	t.Logf("model: %v", model)
}

func Test_model_003(t *testing.T) {
	// Test that GetModel returns an error for an unknown model
	if apiKey == "" {
		t.Skip("MISTRAL_API_KEY not set, skipping")
	}
	assert := assert.New(t)
	client, err := mistral.New(apiKey)
	assert.NoError(err)

	_, err = client.GetModel(context.TODO(), "nonexistent-model-12345")
	assert.Error(err)
}

func Test_model_004(t *testing.T) {
	// Test that model Meta contains capabilities
	if apiKey == "" {
		t.Skip("MISTRAL_API_KEY not set, skipping")
	}
	assert := assert.New(t)
	client, err := mistral.New(apiKey)
	assert.NoError(err)

	model, err := client.GetModel(context.TODO(), "mistral-small-latest")
	assert.NoError(err)
	assert.NotNil(model)
	assert.NotNil(model.Meta)
	assert.Contains(model.Meta, "capabilities")
	assert.Contains(model.Meta, "max_context_length")
	t.Logf("meta: %v", model.Meta)
}
