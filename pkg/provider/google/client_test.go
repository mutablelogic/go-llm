package google_test

import (
	"context"
	"os"
	"testing"

	// Packages
	google "github.com/mutablelogic/go-llm/pkg/provider/google"
	assert "github.com/stretchr/testify/assert"
)

///////////////////////////////////////////////////////////////////////////////
// TEST SET-UP

var (
	client *google.Client
	apiKey string
)

func TestMain(m *testing.M) {
	// API KEY
	apiKey = os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		apiKey = os.Getenv("GOOGLE_API_KEY")
	}
	os.Exit(m.Run())
}

///////////////////////////////////////////////////////////////////////////////
// TESTS

func Test_client_001(t *testing.T) {
	// Test that creating a client with an empty API key succeeds
	// (go-client does not validate the key itself, only the endpoint)
	assert := assert.New(t)
	c, err := google.New("")
	assert.NoError(err)
	assert.NotNil(c)
}

func Test_client_002(t *testing.T) {
	// Test that creating a client with a valid API key succeeds
	if apiKey == "" {
		t.Skip("GEMINI_API_KEY not set, skipping")
	}
	assert := assert.New(t)
	var err error
	client, err = google.New(apiKey)
	assert.NoError(err)
	assert.NotNil(client)
}

func Test_client_003(t *testing.T) {
	// Test that Name() returns the expected provider name
	assert := assert.New(t)
	c, err := google.New("test-key")
	assert.NoError(err)
	assert.Equal("gemini", c.Name())
}

func Test_client_004(t *testing.T) {
	// Test that ListModels returns a non-empty list
	if apiKey == "" {
		t.Skip("GEMINI_API_KEY not set, skipping")
	}
	assert := assert.New(t)
	var err error
	client, err = google.New(apiKey)
	assert.NoError(err)

	models, err := client.ListModels(context.TODO())
	assert.NoError(err)
	assert.NotEmpty(models)

	// Every model should have a name and an owner
	for _, m := range models {
		assert.NotEmpty(m.Name)
		assert.Equal("gemini", m.OwnedBy)
		t.Logf("model: %s (%s)", m.Name, m.Description)
	}
}

func Test_client_005(t *testing.T) {
	// Test that GetModel returns a valid model for a known name
	if apiKey == "" {
		t.Skip("GEMINI_API_KEY not set, skipping")
	}
	assert := assert.New(t)
	var err error
	client, err = google.New(apiKey)
	assert.NoError(err)

	model, err := client.GetModel(context.TODO(), "gemini-2.0-flash")
	assert.NoError(err)
	assert.NotNil(model)
	assert.Contains(model.Name, "gemini-2.0-flash")
	assert.Equal("gemini", model.OwnedBy)
	t.Logf("model: %v", model)
}

func Test_client_006(t *testing.T) {
	// Test that GetModel returns an error for an invalid model name
	if apiKey == "" {
		t.Skip("GEMINI_API_KEY not set, skipping")
	}
	assert := assert.New(t)
	var err error
	client, err = google.New(apiKey)
	assert.NoError(err)

	_, err = client.GetModel(context.TODO(), "nonexistent-model-xyz")
	assert.Error(err)
}
