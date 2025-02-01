package anthropic_test

import (
	"os"
	"testing"

	// Packages
	opts "github.com/mutablelogic/go-client"
	anthropic "github.com/mutablelogic/go-llm/pkg/anthropic"
	assert "github.com/stretchr/testify/assert"
)

func Test_client_001(t *testing.T) {
	assert := assert.New(t)
	client, err := anthropic.New(GetApiKey(t), opts.OptTrace(os.Stderr, true))
	if assert.NoError(err) {
		assert.NotNil(client)
		t.Log(client)
	}
}

///////////////////////////////////////////////////////////////////////////////
// ENVIRONMENT

func GetApiKey(t *testing.T) string {
	key := os.Getenv("ANTHROPIC_API_KEY")
	if key == "" {
		t.Skip("ANTHROPIC_API_KEY not set, skipping tests")
		t.SkipNow()
	}
	return key
}
