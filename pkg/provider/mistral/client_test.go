package mistral_test

import (
	"os"
	"testing"

	// Packages
	mistral "github.com/mutablelogic/go-llm/pkg/provider/mistral"
	assert "github.com/stretchr/testify/assert"
)

///////////////////////////////////////////////////////////////////////////////
// TEST SET-UP

var (
	client *mistral.Client
	apiKey string
)

func TestMain(m *testing.M) {
	// API KEY
	apiKey = os.Getenv("MISTRAL_API_KEY")
	os.Exit(m.Run())
}

///////////////////////////////////////////////////////////////////////////////
// TESTS

func Test_client_001(t *testing.T) {
	// Test that creating a client with an empty API key succeeds
	assert := assert.New(t)
	c, err := mistral.New("")
	assert.NoError(err)
	assert.NotNil(c)
}

func Test_client_002(t *testing.T) {
	// Test that creating a client with a valid API key succeeds
	if apiKey == "" {
		t.Skip("MISTRAL_API_KEY not set, skipping")
	}
	assert := assert.New(t)
	var err error
	client, err = mistral.New(apiKey)
	assert.NoError(err)
	assert.NotNil(client)
}

func Test_client_003(t *testing.T) {
	// Test that Name() returns the expected provider name
	assert := assert.New(t)
	c, err := mistral.New("test-key")
	assert.NoError(err)
	assert.Equal("mistral", c.Name())
}
