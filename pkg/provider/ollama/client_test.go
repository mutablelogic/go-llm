package ollama_test

import (
	"context"
	"os"
	"testing"

	// Packages
	ollama "github.com/mutablelogic/go-llm/pkg/provider/ollama"
	assert "github.com/stretchr/testify/assert"
)

///////////////////////////////////////////////////////////////////////////////
// TEST SET-UP

var (
	client *ollama.Client
	apiKey string
)

func TestMain(m *testing.M) {
	apiKey = os.Getenv("OLLAMA_URL")
	os.Exit(m.Run())
}

///////////////////////////////////////////////////////////////////////////////
// TESTS

func Test_client_001(t *testing.T) {
	// Test that creating a client with no endpoint defaults to localhost
	assert := assert.New(t)
	c, err := ollama.New("")
	assert.NoError(err)
	assert.NotNil(c)
}

func Test_client_002(t *testing.T) {
	// Test that creating a client with a valid API key succeeds
	if apiKey == "" {
		t.Skip("OLLAMA_URL not set, skipping")
	}
	assert := assert.New(t)
	var err error
	client, err = ollama.New(apiKey)
	assert.NoError(err)
	assert.NotNil(client)
}

func Test_client_003(t *testing.T) {
	// Test that Name() returns the expected provider name
	assert := assert.New(t)
	c, err := ollama.New("")
	assert.NoError(err)
	assert.Equal("ollama", c.Name())
}

func Test_client_004(t *testing.T) {
	// Test that ListModels returns a non-empty list
	if apiKey == "" {
		t.Skip("OLLAMA_URL not set, skipping")
	}
	assert := assert.New(t)
	var err error
	client, err = ollama.New(apiKey)
	assert.NoError(err)

	models, err := client.ListModels(context.TODO())
	assert.NoError(err)
	assert.NotEmpty(models)

	// Every model should have a name and an owner
	for _, m := range models {
		assert.NotEmpty(m.Name)
		assert.Equal("ollama", m.OwnedBy)
		t.Logf("model: %s (%s)", m.Name, m.Description)
	}
}

func Test_client_005(t *testing.T) {
	// Test that GetModel returns a valid model for a known name
	if apiKey == "" {
		t.Skip("OLLAMA_URL not set, skipping")
	}
	assert := assert.New(t)
	var err error
	client, err = ollama.New(apiKey)
	assert.NoError(err)

	// Use the first available model from the list
	models, err := client.ListModels(context.TODO())
	if !assert.NoError(err) || !assert.NotEmpty(models) {
		t.FailNow()
	}

	model, err := client.GetModel(context.TODO(), models[0].Name)
	assert.NoError(err)
	if assert.NotNil(model) {
		assert.Equal(models[0].Name, model.Name)
		assert.Equal("ollama", model.OwnedBy)
		t.Logf("model: %v", model)
	}
}
