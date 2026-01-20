package agent_test

import (
	"context"
	"os"
	"testing"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	agent "github.com/mutablelogic/go-llm/pkg/agent"
	assert "github.com/stretchr/testify/assert"
)

func Test_client_001(t *testing.T) {
	assert := assert.New(t)

	opts := []llm.Opt{}
	opts = append(opts, GetOllamaEndpoint(t)...)

	// Create a client
	client, err := agent.New(opts...)
	if assert.NoError(err) {
		assert.NotNil(client)

		// Get models
		models, err := client.Models(context.TODO())
		if !assert.NoError(err) {
			t.FailNow()
		}
		assert.NotNil(models)
		t.Log(models)
	}
}

///////////////////////////////////////////////////////////////////////////////
// ENVIRONMENT

func GetOllamaEndpoint(t *testing.T) []llm.Opt {
	key := os.Getenv("OLLAMA_URL")
	if key == "" {
		return []llm.Opt{}
	} else {
		return []llm.Opt{agent.WithOllama(key)}
	}
}
