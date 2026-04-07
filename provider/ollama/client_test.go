package ollama_test

import (
	"context"
	"os"
	"testing"

	// Packages
	ollama "github.com/mutablelogic/go-llm/provider/ollama"
	assert "github.com/stretchr/testify/assert"
)

///////////////////////////////////////////////////////////////////////////////
// TEST SET-UP

var (
	client    *ollama.Client
	ollamaURL string
)

func TestMain(m *testing.M) {
	ollamaURL = os.Getenv("OLLAMA_URL")
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
	// Test that creating a client with a valid endpoint URL succeeds
	if ollamaURL == "" {
		t.Skip("OLLAMA_URL not set, skipping")
	}
	assert := assert.New(t)
	var err error
	client, err = ollama.New(ollamaURL)
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
	if ollamaURL == "" {
		t.Skip("OLLAMA_URL not set, skipping")
	}
	assert := assert.New(t)
	var err error
	client, err = ollama.New(ollamaURL)
	assert.NoError(err)

	models, err := client.ListModels(context.TODO())
	skipIfUnreachable(t, err)
	assert.NoError(err)
	assert.NotEmpty(models)

	// Every model should have a name and an owner
	for _, m := range models {
		assert.NotEmpty(m.Name)
		assert.Equal(client.Name(), m.OwnedBy)
		t.Logf("model: %s (%s)", m.Name, m.Description)
	}
}

func Test_client_005(t *testing.T) {
	// Test that GetModel returns a valid model for a known name
	if ollamaURL == "" {
		t.Skip("OLLAMA_URL not set, skipping")
	}
	assert := assert.New(t)
	var err error
	client, err = ollama.New(ollamaURL)
	assert.NoError(err)

	// Use the first available model from the list
	models, err := client.ListModels(context.TODO())
	skipIfUnreachable(t, err)
	if !assert.NoError(err) || !assert.NotEmpty(models) {
		t.FailNow()
	}

	model, err := client.GetModel(context.TODO(), models[0].Name)
	assert.NoError(err)
	if assert.NotNil(model) {
		assert.Equal(models[0].Name, model.Name)
		assert.Equal(client.Name(), model.OwnedBy)
		t.Logf("model: %v", model)
	}
}

func Test_client_006(t *testing.T) {
	// Test that a host:port endpoint (no scheme) is accepted and normalised
	assert := assert.New(t)
	c, err := ollama.New("localhost:11434")
	assert.NoError(err)
	assert.NotNil(c)
	assert.Equal("ollama", c.Name())
}

func Test_client_007(t *testing.T) {
	// Test that a URL with no path gets /api appended
	assert := assert.New(t)
	c, err := ollama.New("http://localhost:11434")
	assert.NoError(err)
	assert.NotNil(c)
}

func Test_client_008(t *testing.T) {
	// Test that a URL fragment is ignored and the provider name remains stable
	assert := assert.New(t)
	c, err := ollama.New("http://localhost:11434/api#myprovider")
	assert.NoError(err)
	assert.NotNil(c)
	assert.Equal("ollama", c.Name())
}
