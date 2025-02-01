package mistral_test

import (
	"os"
	"testing"

	// Packages
	opts "github.com/mutablelogic/go-client"
	mistral "github.com/mutablelogic/go-llm/pkg/mistral"
	assert "github.com/stretchr/testify/assert"
)

func Test_client_001(t *testing.T) {
	assert := assert.New(t)
	client, err := mistral.New(GetApiKey(t), opts.OptTrace(os.Stderr, true))
	assert.NoError(err)
	assert.NotNil(client)
	t.Log(client)
}

///////////////////////////////////////////////////////////////////////////////
// ENVIRONMENT

func GetApiKey(t *testing.T) string {
	key := os.Getenv("MISTRAL_API_KEY")
	if key == "" {
		t.Skip("MISTRAL_API_KEY not set")
		t.SkipNow()
	}
	return key
}
